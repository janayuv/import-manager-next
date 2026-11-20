import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/sync_status.dart';

class GDriveService {
  static const String _tokenPrefKey = 'gdrive_access_token';
  static const String _refreshTokenPrefKey = 'gdrive_refresh_token';

  // Integration defaults from environment if passed at build time via --dart-define
  static const String defaultClientId =
      String.fromEnvironment('GDRIVE_CLIENT_ID', defaultValue: '');
  static const String defaultClientSecret =
      String.fromEnvironment('GDRIVE_CLIENT_SECRET', defaultValue: '');
  static const String defaultRefreshToken =
      String.fromEnvironment('GDRIVE_REFRESH_TOKEN', defaultValue: '');
  static const String defaultAccountEmail =
      String.fromEnvironment('GDRIVE_ACCOUNT_EMAIL', defaultValue: '');

  String? _accessToken;
  String? _refreshToken;

  Future<void> init() async {
    final prefs = await SharedPreferences.getInstance();
    _accessToken = prefs.getString(_tokenPrefKey);
    final savedRefresh = prefs.getString(_refreshTokenPrefKey);
    _refreshToken = (savedRefresh != null && savedRefresh.isNotEmpty)
        ? savedRefresh
        : (defaultRefreshToken.isNotEmpty ? defaultRefreshToken : null);
  }

  bool get isAuthenticated => _accessToken != null && _accessToken!.isNotEmpty;

  String? get currentToken => _accessToken;

  Future<void> setAccessToken(String token) async {
    _accessToken = token.trim();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenPrefKey, _accessToken!);
  }

  Future<void> setRefreshToken(String refreshToken) async {
    _refreshToken = refreshToken.trim();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_refreshTokenPrefKey, _refreshToken!);
  }

  /// Automatically requests a fresh Google Drive access token using OAuth refresh token
  Future<String> refreshAccessToken({
    String? refreshToken,
    String? clientId,
    String? clientSecret,
  }) async {
    final rToken = refreshToken ?? _refreshToken ?? defaultRefreshToken;
    final cId = clientId ?? defaultClientId;
    final cSec = clientSecret ?? defaultClientSecret;

    if (rToken.isEmpty || cId.isEmpty || cSec.isEmpty) {
      throw Exception(
        'OAuth credentials not configured. Please paste your Google Drive access token or tap "Pick Backup File".',
      );
    }

    final url = Uri.parse('https://oauth2.googleapis.com/token');
    final response = await http.post(
      url,
      body: {
        'client_id': cId,
        'client_secret': cSec,
        'refresh_token': rToken,
        'grant_type': 'refresh_token',
      },
    );

    if (response.statusCode != 200) {
      throw Exception('OAuth token refresh failed (${response.statusCode}): ${response.body}');
    }

    final data = json.decode(response.body);
    final newToken = data['access_token'] as String?;
    if (newToken == null || newToken.isEmpty) {
      throw Exception('No access_token returned by Google OAuth server');
    }

    await setAccessToken(newToken);
    return newToken;
  }

  Future<void> clearAuth() async {
    _accessToken = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenPrefKey);
  }

  /// Lists backups from Google Drive inside ImportManagerBackups or matching import_manager*.imbk2
  Future<List<DriveBackupFile>> listBackups() async {
    if (!isAuthenticated) {
      try {
        await refreshAccessToken();
      } catch (e) {
        throw Exception('Google Drive not authenticated. Please enter access token or connect desktop account.');
      }
    }

    final query = "name contains 'import_manager' and trashed = false";
    final url = Uri.parse(
      'https://www.googleapis.com/drive/v3/files?q=${Uri.encodeComponent(query)}&fields=files(id,name,size,modifiedTime)&orderBy=modifiedTime desc&pageSize=20',
    );

    var res = await http.get(
      url,
      headers: {
        HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
      },
    );

    // If token expired, auto-refresh and retry once
    if (res.statusCode == 401) {
      await refreshAccessToken();
      res = await http.get(
        url,
        headers: {
          HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
        },
      );
    }

    if (res.statusCode == 401) {
      throw Exception('Google Drive authorization expired (401). Please update token.');
    }
    if (res.statusCode != 200) {
      throw Exception('Failed to list Drive files: ${res.statusCode} ${res.body}');
    }

    final data = json.decode(res.body);
    final filesList = data['files'] as List? ?? [];
    return filesList.map((e) => DriveBackupFile.fromMap(e)).toList();
  }

  /// Downloads backup file payload bytes from Google Drive
  Future<Uint8List> downloadBackupFile(String fileId) async {
    if (!isAuthenticated) {
      await refreshAccessToken();
    }

    final url = Uri.parse('https://www.googleapis.com/drive/v3/files/$fileId?alt=media');
    var res = await http.get(
      url,
      headers: {
        HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
      },
    );

    if (res.statusCode == 401) {
      await refreshAccessToken();
      res = await http.get(
        url,
        headers: {
          HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
        },
      );
    }

    if (res.statusCode != 200) {
      throw Exception('Failed to download file from Drive: ${res.statusCode}');
    }

    return res.bodyBytes;
  }
}
