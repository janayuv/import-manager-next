import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:import_manager_mobile/services/gdrive_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
  });

  test('GDriveService initializes without credentials by default', () async {
    final service = GDriveService();
    await service.init();

    expect(service.hasOAuthCredentials, isFalse);
    expect(service.effectiveClientId.isEmpty, isTrue);
    expect(service.effectiveClientSecret.isEmpty, isTrue);
    expect(service.effectiveRefreshToken.isEmpty, isTrue);
  });

  test('GDriveService recognizes configured OAuth credentials', () async {
    final service = GDriveService();
    await service.init();

    await service.setClientCredentials(
      clientId: 'mock_client_id',
      clientSecret: 'mock_client_secret',
      email: 'user@example.com',
    );
    await service.setRefreshToken('mock_refresh_token');

    expect(service.hasOAuthCredentials, isTrue);
    expect(service.effectiveClientId, equals('mock_client_id'));
    expect(service.effectiveClientSecret, equals('mock_client_secret'));
    expect(service.effectiveRefreshToken, equals('mock_refresh_token'));
    expect(service.effectiveAccountEmail, equals('user@example.com'));
    expect(service.isAuthenticated, isTrue);
  });

  test('GDriveService token expiry detection', () async {
    final service = GDriveService();
    await service.init();
    await service.setClientCredentials(
      clientId: 'mock_client_id',
      clientSecret: 'mock_client_secret',
    );
    await service.setRefreshToken('mock_refresh_token');

    // With no expiry recorded, should report expired so it refreshes cleanly
    expect(service.isTokenExpired, isTrue);

    // Set a token expiring in 1 hour
    final futureExpiry = DateTime.now().millisecondsSinceEpoch + (3600 * 1000);
    await service.setAccessToken('test_token', expiryMs: futureExpiry);
    expect(service.isTokenExpired, isFalse);
    expect(service.currentToken, equals('test_token'));

    // Set a token expiring in 10 seconds (less than 60s buffer)
    final nearExpiry = DateTime.now().millisecondsSinceEpoch + 10000;
    await service.setAccessToken('expiring_token', expiryMs: nearExpiry);
    expect(service.isTokenExpired, isTrue);
  });

  test('GDriveService reset to default credentials', () async {
    final service = GDriveService();
    await service.init();

    await service.setRefreshToken('custom_refresh_token');
    expect(service.effectiveRefreshToken, equals('custom_refresh_token'));

    await service.resetToDefaultCredentials();
    expect(service.effectiveRefreshToken, equals(GDriveService.defaultRefreshToken));
  });
}
