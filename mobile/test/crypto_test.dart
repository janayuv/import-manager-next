import 'dart:convert';
import 'dart:typed_data';
import 'package:flutter_test/flutter_test.dart';
import 'package:cryptography/cryptography.dart';
import 'package:import_manager_mobile/crypto/imbk_decryptor.dart';

void main() {
  test('IMBK2 encryption/decryption round-trip parity test', () async {
    const passphrase = 'SecretPassphrase123!';
    final plaintext = utf8.encode('SQLite format 3\x00 - Test database payload content');

    // Create an IMBK1 format (faster for unit test, 100k iters)
    final salt = Uint8List(16);
    for (int i = 0; i < 16; i++) {
      salt[i] = i + 1;
    }
    final nonce = Uint8List(12);
    for (int i = 0; i < 12; i++) {
      nonce[i] = i + 5;
    }

    final pbkdf2 = Pbkdf2(
      macAlgorithm: Hmac.sha256(),
      iterations: 100000,
      bits: 256,
    );
    final secretKey = await pbkdf2.deriveKeyFromPassword(
      password: passphrase,
      nonce: salt,
    );
    final gcm = AesGcm.with256bits();
    final box = await gcm.encrypt(plaintext, secretKey: secretKey, nonce: nonce);

    final payload = BytesBuilder();
    payload.add(utf8.encode('IMBK1'));
    payload.add(salt);
    payload.add(nonce);
    payload.add(box.cipherText);
    payload.add(box.mac.bytes);

    final encodedPayload = payload.toBytes();
    expect(ImbkDecryptor.isImbkFile(encodedPayload), isTrue);

    final decrypted = await ImbkDecryptor.decrypt(encodedPayload, passphrase);
    expect(utf8.decode(decrypted), utf8.decode(plaintext));
    expect(ImbkDecryptor.isSqliteDatabase(decrypted), isTrue);
  });
}
