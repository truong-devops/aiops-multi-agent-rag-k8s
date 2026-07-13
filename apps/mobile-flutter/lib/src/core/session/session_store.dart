import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../../features/auth/domain/session.dart';

abstract interface class SessionStore {
  Future<Session?> read();

  Future<void> write(Session session);

  Future<void> clear();
}

class SecureSessionStore implements SessionStore {
  const SecureSessionStore({
    FlutterSecureStorage storage = const FlutterSecureStorage(),
  }) : _storage = storage;

  static const _sessionKey = 'aiops_video_mobile.session.v1';

  final FlutterSecureStorage _storage;

  @override
  Future<Session?> read() async {
    final encoded = await _storage.read(key: _sessionKey);
    if (encoded == null || encoded.isEmpty) {
      return null;
    }
    final decoded = jsonDecode(encoded);
    if (decoded is! Map<String, dynamic>) {
      return null;
    }
    return Session.fromJson(decoded);
  }

  @override
  Future<void> write(Session session) {
    return _storage.write(
      key: _sessionKey,
      value: jsonEncode(session.toJson()),
    );
  }

  @override
  Future<void> clear() {
    return _storage.delete(key: _sessionKey);
  }
}
