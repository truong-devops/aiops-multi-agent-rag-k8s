import 'package:flutter/material.dart';

import '../../../shared/widgets/app_empty_state.dart';
import '../../../shared/widgets/app_section.dart';
import '../../../shared/widgets/status_chip.dart';
import '../data/feed_repository.dart';
import '../domain/feed_item.dart';

class FeedScreen extends StatefulWidget {
  const FeedScreen({super.key, required this.repository});

  final FeedRepository repository;

  @override
  State<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends State<FeedScreen> {
  late Future<List<FeedItem>> _future = widget.repository.listFeed();

  Future<void> _refresh() async {
    setState(() {
      _future = widget.repository.listFeed();
    });
    await _future;
  }

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _refresh,
      child: FutureBuilder<List<FeedItem>>(
        future: _future,
        builder: (context, snapshot) {
          final items = snapshot.data ?? const <FeedItem>[];
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              const AppSectionHeader(
                title: 'Feed',
                subtitle: 'Ready videos from feed-social-service.',
              ),
              const SizedBox(height: 12),
              if (snapshot.connectionState == ConnectionState.waiting)
                const LinearProgressIndicator(),
              if (snapshot.hasError)
                AppEmptyState(label: snapshot.error.toString()),
              if (!snapshot.hasError &&
                  items.isEmpty &&
                  snapshot.connectionState != ConnectionState.waiting)
                const AppEmptyState(label: 'No ready videos yet.'),
              ...items.map((item) => _FeedCard(item: item)),
            ],
          );
        },
      ),
    );
  }
}

class _FeedCard extends StatelessWidget {
  const _FeedCard({required this.item});

  final FeedItem item;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              height: 210,
              decoration: BoxDecoration(
                color: const Color(0xFF111827),
                borderRadius: BorderRadius.circular(12),
              ),
              child: const Center(
                child: Icon(Icons.play_arrow_rounded,
                    size: 58, color: Colors.white),
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: Text(
                    item.title.isEmpty ? item.videoId : item.title,
                    style: const TextStyle(
                        fontWeight: FontWeight.w800, fontSize: 16),
                  ),
                ),
                const StatusChip(label: 'ready', tone: StatusTone.ready),
              ],
            ),
            if (item.description.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(item.description,
                  style: const TextStyle(color: Color(0xFF6B7280))),
            ],
            const SizedBox(height: 10),
            Text(
              '${item.likeCount} likes · ${item.commentCount} comments · ${_duration(item.durationMs)}',
              style: const TextStyle(color: Color(0xFF6B7280)),
            ),
          ],
        ),
      ),
    );
  }

  String _duration(int durationMs) {
    if (durationMs <= 0) {
      return '-';
    }
    final seconds = (durationMs / 1000).round();
    final minutes = seconds ~/ 60;
    final rest = seconds % 60;
    return '$minutes:${rest.toString().padLeft(2, '0')}';
  }
}
