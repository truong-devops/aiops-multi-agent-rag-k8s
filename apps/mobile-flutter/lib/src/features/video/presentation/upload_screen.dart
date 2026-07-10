import 'package:flutter/material.dart';

import '../../../core/session/session_controller.dart';
import '../../../shared/widgets/app_empty_state.dart';
import '../../../shared/widgets/app_section.dart';
import '../../../shared/widgets/status_chip.dart';
import '../data/video_repository.dart';
import '../domain/video_item.dart';

class UploadScreen extends StatefulWidget {
  const UploadScreen({
    super.key,
    required this.repository,
    required this.sessionController,
  });

  final VideoRepository repository;
  final SessionController sessionController;

  @override
  State<UploadScreen> createState() => _UploadScreenState();
}

class _UploadScreenState extends State<UploadScreen> {
  final _title = TextEditingController();
  final _description = TextEditingController();
  final _sizeBytes = TextEditingController(text: '1048576');
  final _checksum = TextEditingController(text: 'demo-checksum');
  String _visibility = 'public';
  String? _message;
  bool _loading = false;
  late Future<List<VideoItem>> _videosFuture;

  @override
  void initState() {
    super.initState();
    widget.sessionController.addListener(_loadVideos);
    _videosFuture = _buildVideosFuture();
  }

  @override
  void dispose() {
    widget.sessionController.removeListener(_loadVideos);
    _title.dispose();
    _description.dispose();
    _sizeBytes.dispose();
    _checksum.dispose();
    super.dispose();
  }

  void _loadVideos() {
    setState(() {
      _videosFuture = _buildVideosFuture();
    });
  }

  Future<List<VideoItem>> _buildVideosFuture() {
    final session = widget.sessionController.session;
    return session == null
        ? Future.value(const <VideoItem>[])
        : widget.repository.listMyVideos(session.accessToken);
  }

  Future<void> _createIntent() async {
    final session = widget.sessionController.session;
    if (session == null) {
      setState(() => _message = 'Sign in before uploading.');
      return;
    }

    setState(() {
      _loading = true;
      _message = null;
    });
    try {
      final intent = await widget.repository.createUploadIntent(
        token: session.accessToken,
        title: _title.text.trim(),
        description: _description.text.trim(),
        visibility: _visibility,
        contentType: 'video/mp4',
        sizeBytes: int.tryParse(_sizeBytes.text.trim()) ?? 0,
        checksumSha256: _checksum.text.trim(),
      );
      setState(() {
        _message = 'Upload URL ready for ${intent.videoId}.';
      });
      _loadVideos();
    } catch (error) {
      setState(() => _message = error.toString());
    } finally {
      if (mounted) {
        setState(() => _loading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        const AppSectionHeader(
          title: 'Upload',
          subtitle: 'Create video upload intents.',
        ),
        const SizedBox(height: 12),
        AppSection(
          title: 'New video',
          subtitle: 'File picker integration can be added after platform setup.',
          child: Column(
            children: [
              TextField(controller: _title, decoration: const InputDecoration(labelText: 'Title')),
              const SizedBox(height: 10),
              TextField(
                controller: _description,
                minLines: 2,
                maxLines: 3,
                decoration: const InputDecoration(labelText: 'Description'),
              ),
              const SizedBox(height: 10),
              DropdownButtonFormField<String>(
                value: _visibility,
                decoration: const InputDecoration(labelText: 'Visibility'),
                items: const [
                  DropdownMenuItem(value: 'public', child: Text('Public')),
                  DropdownMenuItem(value: 'private', child: Text('Private')),
                ],
                onChanged: (value) => setState(() => _visibility = value ?? 'public'),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _sizeBytes,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(labelText: 'Size bytes'),
              ),
              const SizedBox(height: 10),
              TextField(controller: _checksum, decoration: const InputDecoration(labelText: 'SHA-256 checksum')),
              if (_message != null) ...[
                const SizedBox(height: 10),
                Text(_message!, style: const TextStyle(color: Color(0xFF374151))),
              ],
              const SizedBox(height: 14),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: _loading ? null : _createIntent,
                  icon: const Icon(Icons.cloud_upload_outlined),
                  label: Text(_loading ? 'Creating...' : 'Create upload intent'),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        AppSection(
          title: 'My videos',
          subtitle: 'Processing status from video-service.',
          child: FutureBuilder<List<VideoItem>>(
            future: _videosFuture,
            builder: (context, snapshot) {
              final items = snapshot.data ?? const <VideoItem>[];
              if (snapshot.connectionState == ConnectionState.waiting) {
                return const LinearProgressIndicator();
              }
              if (snapshot.hasError) {
                return AppEmptyState(label: snapshot.error.toString());
              }
              if (items.isEmpty) {
                return const AppEmptyState(label: 'No videos yet.');
              }
              return Column(
                children: items.map((video) => _VideoRow(video: video)).toList(),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _VideoRow extends StatelessWidget {
  const _VideoRow({required this.video});

  final VideoItem video;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: const Icon(Icons.movie_outlined),
      title: Text(video.title.isEmpty ? video.id : video.title),
      subtitle: Text(video.id),
      trailing: StatusChip.fromStatus(video.status),
    );
  }
}
