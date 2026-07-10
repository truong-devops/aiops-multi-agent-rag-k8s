import 'package:flutter/foundation.dart';

import '../../features/auth/domain/session.dart';

class SessionController extends ChangeNotifier {
  Session? _session;

  Session? get session => _session;
  bool get isSignedIn => _session != null;

  void setSession(Session session) {
    _session = session;
    notifyListeners();
  }

  void clear() {
    _session = null;
    notifyListeners();
  }
}
