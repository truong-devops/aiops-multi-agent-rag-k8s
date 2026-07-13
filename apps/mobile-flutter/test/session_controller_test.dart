import 'package:aiops_video_mobile/src/core/session/session_controller.dart';
import 'package:aiops_video_mobile/src/core/session/session_store.dart';
import 'package:aiops_video_mobile/src/features/auth/domain/session.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('restores, saves, and clears session state', () async {
    final store = _MemorySessionStore();
    final controller = SessionController(store: store);

    expect(controller.isRestoring, isTrue);
    expect(controller.isSignedIn, isFalse);

    await controller.restore();

    expect(controller.isRestoring, isFalse);
    expect(controller.isSignedIn, isFalse);

    await controller.setSession(_session);

    expect(controller.isSignedIn, isTrue);
    expect(controller.session?.user.email, 'viewer@example.com');
    expect(store.session?.accessToken, 'access-token');

    final restored = SessionController(store: store);
    await restored.restore();

    expect(restored.isSignedIn, isTrue);
    expect(restored.session?.user.id, 'user-1');

    await restored.clear();

    expect(restored.isSignedIn, isFalse);
    expect(store.session, isNull);
  });
}

const _session = Session(
  accessToken: 'access-token',
  refreshToken: 'refresh-token',
  tokenType: 'Bearer',
  expiresIn: 3600,
  user: User(
    id: 'user-1',
    email: 'viewer@example.com',
    displayName: 'Viewer',
    roles: ['user'],
  ),
);

class _MemorySessionStore implements SessionStore {
  Session? session;

  @override
  Future<Session?> read() async => session;

  @override
  Future<void> write(Session session) async {
    this.session = session;
  }

  @override
  Future<void> clear() async {
    session = null;
  }
}
