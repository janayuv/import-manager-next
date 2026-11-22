import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/sync_status.dart';

class GDriveService {
  static const String _tokenPrefKey = 'gdrive_access_token';
  static const String _refreshTokenPrefKey = 'gdrive_refresh_token';
  static const String _tokenExpiryPrefKey = 'gdrive_token_expiry_ms';
  static const String _clientIdPrefKey = 'gdrive_client_id';
  static const String _clientSecretPrefKey = 'gdrive_client_secret';
  static const String _accountEmailPrefKey = 'gdrive_account_email';

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
  int? _tokenExpiryEpochMs;
  String? _clientId;
  String? _clientSecret;
  String? _accountEmail;

  Future<void> init() async {
    final prefs = await SharedPreferences.getInstance();
    _accessToken = prefs.getString(_tokenPrefKey);
    final savedRefresh = prefs.getString(_refreshTokenPrefKey);
    _refreshToken = (savedRefresh != null && savedRefresh.isNotEmpty)
        ? savedRefresh
        : (defaultRefreshToken.isNotEmpty ? defaultRefreshToken : null);

    _tokenExpiryEpochMs = prefs.getInt(_tokenExpiryPrefKey);

    final savedClientId = prefs.getString(_clientIdPrefKey);
    _clientId = (savedClientId != null && savedClientId.isNotEmpty)
        ? savedClientId
        : (defaultClientId.isNotEmpty ? defaultClientId : null);

    final savedClientSecret = prefs.getString(_clientSecretPrefKey);
    _clientSecret = (savedClientSecret != null && savedClientSecret.isNotEmpty)
        ? savedClientSecret
        : (defaultClientSecret.isNotEmpty ? defaultClientSecret : null);

    final savedEmail = prefs.getString(_accountEmailPrefKey);
    _accountEmail = (savedEmail != null && savedEmail.isNotEmpty)
        ? savedEmail
        : (defaultAccountEmail.isNotEmpty ? defaultAccountEmail : null);
  }

  String get effectiveRefreshToken => (_refreshToken != null && _refreshToken!.isNotEmpty)
      ? _refreshToken!
      : defaultRefreshToken;

  String get effectiveClientId => (_clientId != null && _clientId!.isNotEmpty)
      ? _clientId!
      : defaultClientId;

  String get effectiveClientSecret => (_clientSecret != null && _clientSecret!.isNotEmpty)
      ? _clientSecret!
      : defaultClientSecret;

  String get effectiveAccountEmail => (_accountEmail != null && _accountEmail!.isNotEmpty)
      ? _accountEmail!
      : defaultAccountEmail;

  bool get hasOAuthCredentials =>
      effectiveRefreshToken.isNotEmpty &&
      effectiveClientId.isNotEmpty &&
      effectiveClientSecret.isNotEmpty;

  /// Returns true if the cached token is expired or will expire within 60 seconds
  bool get isTokenExpired {
    if (_tokenExpiryEpochMs == null) {
      // If expiry is unknown and OAuth credentials exist, treat as expired so we get a fresh known token
      return hasOAuthCredentials;
    }
    return DateTime.now().millisecondsSinceEpoch >= (_tokenExpiryEpochMs! - 60000);
  }

  /// True if we either have a fresh access token OR valid OAuth refresh credentials
  bool get isAuthenticated =>
      (_accessToken != null && _accessToken!.isNotEmpty && !isTokenExpired) ||
      hasOAuthCredentials;

  String? get currentToken => _accessToken;

  Future<void> setAccessToken(String token, {int? expiryMs}) async {
    _accessToken = token.trim();
    _tokenExpiryEpochMs = expiryMs;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenPrefKey, _accessToken!);
    if (expiryMs != null) {
      await prefs.setInt(_tokenExpiryPrefKey, expiryMs);
    } else {
      await prefs.remove(_tokenExpiryPrefKey);
    }
  }

  Future<void> setRefreshToken(String refreshToken) async {
    _refreshToken = refreshToken.trim();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_refreshTokenPrefKey, _refreshToken!);
  }

  Future<void> setClientCredentials({
    required String clientId,
    required String clientSecret,
    String? email,
  }) async {
    _clientId = clientId.trim();
    _clientSecret = clientSecret.trim();
    if (email != null && email.isNotEmpty) {
      _accountEmail = email.trim();
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_clientIdPrefKey, _clientId!);
    await prefs.setString(_clientSecretPrefKey, _clientSecret!);
    if (_accountEmail != null) {
      await prefs.setString(_accountEmailPrefKey, _accountEmail!);
    }
  }

  /// Ensures a valid, unexpired Google Drive access token is available.
  /// Automatically calls refreshAccessToken() whenever the token is missing or expired.
  Future<String> ensureValidAccessToken() async {
    if (_accessToken != null && _accessToken!.isNotEmpty && !isTokenExpired) {
      return _accessToken!;
    }
    if (hasOAuthCredentials) {
      return await refreshAccessToken();
    }
    if (_accessToken != null && _accessToken!.isNotEmpty) {
      return _accessToken!;
    }
    throw Exception(
      'OAuth credentials not configured. Please tap "Pick Backup File" or configure Google Drive credentials.',
    );
  }

  /// Automatically requests a fresh Google Drive access token using OAuth refresh token
  Future<String> refreshAccessToken({
    String? refreshToken,
    String? clientId,
    String? clientSecret,
  }) async {
    final rToken = (refreshToken != null && refreshToken.isNotEmpty)
        ? refreshToken
        : effectiveRefreshToken;
    final cId = (clientId != null && clientId.isNotEmpty)
        ? clientId
        : effectiveClientId;
    final cSec = (clientSecret != null && clientSecret.isNotEmpty)
        ? clientSecret
        : effectiveClientSecret;

    if (rToken.isEmpty || cId.isEmpty || cSec.isEmpty) {
      throw Exception(
        'OAuth credentials not configured. Please tap "Pick Backup File" or configure Google Drive credentials.',
      );
    }

    final url = Uri.parse('https://oauth2.googleapis.com/token');
    final response = await http.post(
      url,
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
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
    final expiresIn = (data['expires_in'] as num?)?.toInt() ?? 3600;

    if (newToken == null || newToken.isEmpty) {
      throw Exception('No access_token returned by Google OAuth server');
    }

    final expiryMs = DateTime.now().millisecondsSinceEpoch + (expiresIn * 1000);
    await setAccessToken(newToken, expiryMs: expiryMs);
    return newToken;
  }

  Future<void> clearAuth() async {
    _accessToken = null;
    _tokenExpiryEpochMs = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenPrefKey);
    await prefs.remove(_tokenExpiryPrefKey);
  }

  Future<void> resetToDefaultCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenPrefKey);
    await prefs.remove(_refreshTokenPrefKey);
    await prefs.remove(_tokenExpiryPrefKey);
    await prefs.remove(_clientIdPrefKey);
    await prefs.remove(_clientSecretPrefKey);
    await prefs.remove(_accountEmailPrefKey);
    _accessToken = null;
    _refreshToken = null;
    _tokenExpiryEpochMs = null;
    _clientId = null;
    _clientSecret = null;
    _accountEmail = null;
    await init();
  }

  /// Lists backups from Google Drive matching import_manager*.imbk2
  Future<List<DriveBackupFile>> listBackups() async {
    await ensureValidAccessToken();

    final query = "name contains 'import_manager' and mimeType != 'application/vnd.google-apps.folder' and trashed = false";
    final url = Uri.parse(
      'https://www.googleapis.com/drive/v3/files?q=${Uri.encodeComponent(query)}&fields=files(id,name,size,modifiedTime)&orderBy=modifiedTime desc&pageSize=20',
    );

    var res = await http.get(
      url,
      headers: {
        HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
      },
    );

    // If token expired (401), auto-refresh and retry once
    if (res.statusCode == 401 && hasOAuthCredentials) {
      await refreshAccessToken();
      res = await http.get(
        url,
        headers: {
          HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
        },
      );
    }

    if (res.statusCode == 403) {
      throw Exception(
        'Google Drive Error 403 (Permission Denied): Google Drive API is not enabled in Google Cloud Console for project 874914396315. '
        'Please switch to the "Email & Password Connect" option to sync directly with Desktop, or enable Google Drive API in Google Cloud Console.',
      );
    }
    if (res.statusCode == 401) {
      throw Exception('Google Drive authorization expired (401). Please verify credentials or switch to Email & Password connect.');
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
    await ensureValidAccessToken();

    final url = Uri.parse('https://www.googleapis.com/drive/v3/files/$fileId?alt=media');
    var res = await http.get(
      url,
      headers: {
        HttpHeaders.authorizationHeader: 'Bearer $_accessToken',
      },
    );

    if (res.statusCode == 401 && hasOAuthCredentials) {
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
