import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:import_manager_mobile/services/mobile_sync_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
  });

  test('MobileSyncService URL normalization tests', () {
    final service = MobileSyncService();

    expect(service.normalizeUrl('192.168.1.5'), equals('http://192.168.1.5:8766'));
    expect(service.normalizeUrl('http://192.168.1.5:8766/'), equals('http://192.168.1.5:8766'));
    expect(service.normalizeUrl('https://sync.example.com'), equals('https://sync.example.com'));
    expect(service.normalizeUrl(''), equals('http://127.0.0.1:8766'));
  });

  test('MobileSyncService initial state', () async {
    final service = MobileSyncService();
    await service.init();

    expect(service.isAuthenticated, isFalse);
    expect(service.effectiveEmail, isEmpty);
    expect(service.effectiveServerUrl, equals('http://192.168.1.100:8766'));
  });

  test('MobileSyncService remembers credentials after save', () async {
    SharedPreferences.setMockInitialValues({
      'direct_sync_server_url': 'http://192.168.1.20:8766',
      'direct_sync_email': 'test@gmail.com',
      'direct_sync_auth_token': 'token_xyz_123',
    });

    final service = MobileSyncService();
    await service.init();

    expect(service.isAuthenticated, isTrue);
    expect(service.effectiveEmail, equals('test@gmail.com'));
    expect(service.effectiveServerUrl, equals('http://192.168.1.20:8766'));

    await service.clearAuth();
    expect(service.isAuthenticated, isFalse);
  });
}
