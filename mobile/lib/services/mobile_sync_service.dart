import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

class MobileSyncResult {
  final Uint8List bytes;
  final String filename;
  final String companyName;

  MobileSyncResult({
    required this.bytes,
    required this.filename,
    required this.companyName,
  });
}

class MobileSyncService {
  static const String _prefServerUrl = 'direct_sync_server_url';
  static const String _prefEmail = 'direct_sync_email';
  static const String _prefAuthToken = 'direct_sync_auth_token';
  static const String _prefLastSyncMode = 'sync_active_mode'; // 'direct', 'gdrive', 'file'

  // Default desktop companion sync port
  static const String defaultPort = '8766';

  String? _serverUrl;
  String? _email;
  String? _authToken;

  Future<void> init() async {
    final prefs = await SharedPreferences.getInstance();
    _serverUrl = prefs.getString(_prefServerUrl);
    _email = prefs.getString(_prefEmail);
    _authToken = prefs.getString(_prefAuthToken);
  }

  String get effectiveServerUrl {
    if (_serverUrl != null && _serverUrl!.trim().isNotEmpty) {
      return _serverUrl!.trim();
    }
    return 'http://192.168.1.100:$defaultPort';
  }

  String get effectiveEmail => _email ?? '';
  bool get isAuthenticated => _authToken != null && _authToken!.isNotEmpty;

  String normalizeUrl(String input) {
    var url = input.trim();
    if (url.isEmpty) return 'http://127.0.0.1:$defaultPort';
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      url = 'http://$url';
    }
    final uri = Uri.tryParse(url);
    if (uri != null && !uri.hasPort && uri.scheme == 'http') {
      final path = uri.path.isNotEmpty ? uri.path : '';
      url = '${uri.scheme}://${uri.host}:$defaultPort$path';
    }
    return url.endsWith('/') ? url.substring(0, url.length - 1) : url;
  }

  /// Tests connectivity to desktop sync server
  Future<Map<String, dynamic>> testConnection(String serverUrl) async {
    final base = normalizeUrl(serverUrl);
    final url = Uri.parse('$base/api/mobile/status');
    final response = await http.get(url).timeout(const Duration(seconds: 5));

    if (response.statusCode != 200) {
      throw Exception('Server responded with status ${response.statusCode}: ${response.body}');
    }
    return json.decode(response.body) as Map<String, dynamic>;
  }

  /// Logs in with Email/Username and Password, saving session token
  Future<String> login({
    required String serverUrl,
    required String email,
    required String password,
  }) async {
    final base = normalizeUrl(serverUrl);
    final url = Uri.parse('$base/api/mobile/login');

    final response = await http.post(
      url,
      headers: {
        HttpHeaders.contentTypeHeader: 'application/json',
      },
      body: json.encode({
        'email': email.trim(),
        'username': email.trim(),
        'password': password.trim(),
      }),
    ).timeout(const Duration(seconds: 8));

    if (response.statusCode != 200) {
      String errMsg = 'Authentication failed (${response.statusCode})';
      try {
        final data = json.decode(response.body);
        if (data['error'] != null) errMsg = data['error'];
      } catch (_) {}
      throw Exception(errMsg);
    }

    final data = json.decode(response.body);
    final token = data['token'] as String? ?? '';
    if (token.isEmpty) {
      throw Exception('Server did not return a valid session token');
    }

    _serverUrl = base;
    _email = email.trim();
    _authToken = token;

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefServerUrl, base);
    await prefs.setString(_prefEmail, _email!);
    await prefs.setString(_prefAuthToken, token);
    await prefs.setString(_prefLastSyncMode, 'direct');

    return token;
  }

  /// Fetches the latest .imbk2 backup payload directly from desktop server
  Future<MobileSyncResult> fetchLatestBackup({
    String? serverUrl,
    String? email,
    String? password,
  }) async {
    String base = serverUrl != null && serverUrl.trim().isNotEmpty
        ? normalizeUrl(serverUrl)
        : effectiveServerUrl;

    // If password provided, perform fresh login
    if (password != null && password.isNotEmpty && email != null && email.isNotEmpty) {
      await login(serverUrl: base, email: email, password: password);
    }

    if (_authToken == null || _authToken!.isEmpty) {
      throw Exception('Not authenticated. Please enter Email and Password to connect.');
    }

    final url = Uri.parse('$base/api/mobile/sync/latest');
    var response = await http.get(
      url,
      headers: {
        HttpHeaders.authorizationHeader: 'Bearer $_authToken',
        'X-Auth-Token': _authToken!,
      },
    ).timeout(const Duration(seconds: 30));

    // If session expired (401) and credentials are known, auto-relogin if password was provided
    if (response.statusCode == 401 && password != null && password.isNotEmpty && email != null) {
      await login(serverUrl: base, email: email, password: password);
      response = await http.get(
        url,
        headers: {
          HttpHeaders.authorizationHeader: 'Bearer $_authToken',
          'X-Auth-Token': _authToken!,
        },
      ).timeout(const Duration(seconds: 30));
    }

    if (response.statusCode == 401) {
      throw Exception('Session expired or invalid credentials. Please log in again.');
    }
    if (response.statusCode != 200) {
      throw Exception('Failed to download live backup from desktop: ${response.statusCode}');
    }

    String filename = response.headers['x-backup-filename'] ?? '';
    if (filename.isEmpty) {
      final cd = response.headers['content-disposition'] ?? '';
      final match = RegExp(r'filename="?([^"]+)"?').firstMatch(cd);
      if (match != null) filename = match.group(1)!;
    }
    if (filename.isEmpty) {
      filename = 'desktop_live_sync_${DateTime.now().millisecondsSinceEpoch}.imbk2';
    }

    return MobileSyncResult(
      bytes: response.bodyBytes,
      filename: filename,
      companyName: '',
    );
  }

  Future<void> clearAuth() async {
    _authToken = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_prefAuthToken);
  }
}
