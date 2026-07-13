import 'package:flutter/material.dart';

import '../core/di/app_dependencies.dart';
import '../features/auth/presentation/auth_sheet.dart';
import '../features/auth/domain/session.dart';
import '../features/feed/presentation/feed_screen.dart';
import '../features/live/presentation/live_screen.dart';
import '../features/profile/presentation/profile_screen.dart';
import '../features/video/presentation/upload_screen.dart';

class AppShell extends StatefulWidget {
  const AppShell({super.key, required this.dependencies});

  final AppDependencies dependencies;

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  int _index = 0;

  @override
  void initState() {
    super.initState();
    widget.dependencies.sessionController.addListener(_onSessionChanged);
  }

  @override
  void dispose() {
    widget.dependencies.sessionController.removeListener(_onSessionChanged);
    super.dispose();
  }

  void _onSessionChanged() {
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final dependencies = widget.dependencies;
    final sessionController = dependencies.sessionController;
    final session = sessionController.session;
    final isRestoringSession = sessionController.isRestoring;
    final pages = [
      FeedScreen(repository: dependencies.feedRepository),
      UploadScreen(
        repository: dependencies.videoRepository,
        sessionController: sessionController,
      ),
      LiveScreen(
        repository: dependencies.liveRepository,
        sessionController: sessionController,
      ),
      ProfileScreen(
        config: dependencies.config,
        sessionController: sessionController,
      ),
    ];

    return Scaffold(
      appBar: AppBar(
        title: const _AppTitle(),
        actions: [
          TextButton.icon(
            onPressed: isRestoringSession ? null : () => _openAuth(context),
            icon: Icon(
              isRestoringSession
                  ? Icons.hourglass_empty
                  : session == null
                      ? Icons.login
                      : Icons.verified_user_outlined,
            ),
            label: Text(
              isRestoringSession
                  ? 'Checking'
                  : session == null
                      ? 'Sign in'
                      : 'Signed in',
            ),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: SafeArea(
        child: IndexedStack(index: _index, children: pages),
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (value) => setState(() => _index = value),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.play_circle_outline),
            selectedIcon: Icon(Icons.play_circle),
            label: 'Feed',
          ),
          NavigationDestination(
            icon: Icon(Icons.cloud_upload_outlined),
            selectedIcon: Icon(Icons.cloud_upload),
            label: 'Upload',
          ),
          NavigationDestination(
            icon: Icon(Icons.sensors_outlined),
            selectedIcon: Icon(Icons.sensors),
            label: 'Live',
          ),
          NavigationDestination(
            icon: Icon(Icons.person_outline),
            selectedIcon: Icon(Icons.person),
            label: 'Profile',
          ),
        ],
      ),
    );
  }

  Future<void> _openAuth(BuildContext context) async {
    final session = await showModalBottomSheet<Session>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (context) =>
          AuthSheet(repository: widget.dependencies.authRepository),
    );
    if (session != null) {
      await widget.dependencies.sessionController.setSession(session);
    }
  }
}

class _AppTitle extends StatelessWidget {
  const _AppTitle();

  @override
  Widget build(BuildContext context) {
    return const Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'AIOps Video',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800),
        ),
        Text(
          'Viewer demo',
          style: TextStyle(fontSize: 12, color: Color(0xFF6B7280)),
        ),
      ],
    );
  }
}
