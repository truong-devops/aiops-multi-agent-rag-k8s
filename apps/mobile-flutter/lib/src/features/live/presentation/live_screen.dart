import 'package:flutter/material.dart';

import '../../../core/session/session_controller.dart';
import '../../../shared/widgets/app_empty_state.dart';
import '../../../shared/widgets/app_section.dart';
import '../../../shared/widgets/status_chip.dart';
import '../data/live_repository.dart';
import '../domain/live_session.dart';

class LiveScreen extends StatefulWidget {
  const LiveScreen({
    super.key,
    required this.repository,
    required this.sessionController,
  });

  final LiveRepository repository;
  final SessionController sessionController;

  @override
  State<LiveScreen> createState() => _LiveScreenState();
}

class _LiveScreenState extends State<LiveScreen> {
  late Future<List<LiveSession>> _future = _load();

  @override
  void initState() {
    super.initState();
    widget.sessionController.addListener(_reload);
  }

  @override
  void dispose() {
    widget.sessionController.removeListener(_reload);
    super.dispose();
  }

  Future<List<LiveSession>> _load() {
    final session = widget.sessionController.session;
    if (session == null) {
      return Future.value(const <LiveSession>[]);
    }
    return widget.repository.listLiveSessions(token: session.accessToken);
  }

  void _reload() {
    setState(() => _future = _load());
  }

  Future<void> _refresh() async {
    _reload();
    await _future;
  }

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _refresh,
      child: FutureBuilder<List<LiveSession>>(
        future: _future,
        builder: (context, snapshot) {
          final items = snapshot.data ?? const <LiveSession>[];
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              const AppSectionHeader(
                title: 'Live',
                subtitle: 'Watch active creator sessions.',
              ),
              const SizedBox(height: 12),
              if (snapshot.connectionState == ConnectionState.waiting)
                const LinearProgressIndicator(),
              if (snapshot.hasError)
                AppEmptyState(label: snapshot.error.toString()),
              if (items.isEmpty &&
                  snapshot.connectionState != ConnectionState.waiting)
                AppEmptyState(
                  label: widget.sessionController.session == null
                      ? 'Sign in to view live sessions.'
                      : 'No sessions yet.',
                ),
              ...items.map((session) => _LiveCard(session: session)),
            ],
          );
        },
      ),
    );
  }
}

class _LiveCard extends StatelessWidget {
  const _LiveCard({required this.session});

  final LiveSession session;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: ListTile(
        leading: const Icon(Icons.sensors),
        title: Text(session.title.isEmpty ? session.id : session.title),
        subtitle: Text(
            session.playbackUrl.isEmpty ? session.id : session.playbackUrl),
        trailing: StatusChip.fromStatus(session.status),
      ),
    );
  }
}
