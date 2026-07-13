import 'package:flutter/foundation.dart';

import '../../features/auth/domain/session.dart';
import 'session_store.dart';

class SessionController extends ChangeNotifier {
  SessionController({required SessionStore store}) : _store = store;

  final SessionStore _store;

  Session? _session;
  bool _isRestoring = true;
  String? _storageError;

  Session? get session => _session;
  bool get isSignedIn => _session != null;
  bool get isRestoring => _isRestoring;
  String? get storageError => _storageError;

  Future<void> restore() async {
    try {
      _session = await _store.read();
      _storageError = null;
    } catch (error) {
      _storageError = 'Could not restore saved session.';
    } finally {
      _isRestoring = false;
      notifyListeners();
    }
  }

  Future<void> setSession(Session session) async {
    _session = session;
    _storageError = null;
    notifyListeners();
    try {
      await _store.write(session);
    } catch (error) {
      _storageError = 'Signed in, but session could not be saved.';
      notifyListeners();
    }
  }

  Future<void> clear() async {
    _session = null;
    _storageError = null;
    notifyListeners();
    try {
      await _store.clear();
    } catch (error) {
      _storageError =
          'Signed out locally, but saved session could not be cleared.';
      notifyListeners();
    }
  }
}
