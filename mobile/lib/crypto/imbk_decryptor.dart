import 'dart:typed_data';
import 'package:cryptography/cryptography.dart';

class ImbkDecryptor {
  static const String magicV2 = 'IMBK2';
  static const String magicV1 = 'IMBK1';
  static const int saltLength = 16;
  static const int nonceLength = 12;
  static const int tagLength = 16;
  static const int itersV2 = 600000;
  static const int itersV1 = 100000;

  static bool isImbkFile(Uint8List bytes) {
    if (bytes.length < 5) return false;
    final header = String.fromCharCodes(bytes.sublist(0, 5));
    return header == magicV2 || header == magicV1;
  }

  static bool isSqliteDatabase(Uint8List bytes) {
    if (bytes.length < 16) return false;
    // Standard SQLite header: "SQLite format 3\000"
    final header = String.fromCharCodes(bytes.sublist(0, 15));
    return header == 'SQLite format 3';
  }

  static Future<Uint8List> decrypt(Uint8List payload, String passphrase) async {
    if (payload.length < 5 + saltLength + nonceLength + tagLength) {
      throw Exception('Backup file payload is truncated or too small');
    }

    final header = String.fromCharCodes(payload.sublist(0, 5));
    int iterations;
    if (header == magicV2) {
      iterations = itersV2;
    } else if (header == magicV1) {
      iterations = itersV1;
    } else {
      throw Exception('Unsupported backup header "$header". Expected IMBK2 or IMBK1.');
    }

    final salt = payload.sublist(5, 5 + saltLength);
    final nonce = payload.sublist(5 + saltLength, 5 + saltLength + nonceLength);
    final cipherAndTag = payload.sublist(5 + saltLength + nonceLength);

    if (cipherAndTag.length < tagLength) {
      throw Exception('Invalid ciphertext: missing GCM authentication tag');
    }

    final cipherText = cipherAndTag.sublist(0, cipherAndTag.length - tagLength);
    final macTag = cipherAndTag.sublist(cipherAndTag.length - tagLength);

    final pbkdf2 = Pbkdf2(
      macAlgorithm: Hmac.sha256(),
      iterations: iterations,
      bits: 256,
    );

    final secretKey = await pbkdf2.deriveKeyFromPassword(
      password: passphrase,
      nonce: salt,
    );

    final gcm = AesGcm.with256bits();
    final secretBox = SecretBox(
      cipherText,
      nonce: nonce,
      mac: Mac(macTag),
    );

    try {
      final decrypted = await gcm.decrypt(
        secretBox,
        secretKey: secretKey,
      );
      return Uint8List.fromList(decrypted);
    } catch (e) {
      throw Exception('Decryption failed: Incorrect backup passphrase or corrupted file.');
    }
  }
}
