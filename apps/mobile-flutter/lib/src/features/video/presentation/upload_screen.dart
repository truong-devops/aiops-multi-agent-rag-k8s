import 'package:crypto/crypto.dart';
import 'package:file_picker/file_picker.dart';
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
  String _visibility = 'public';
  String? _message;
  PlatformFile? _selectedFile;
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

  Future<void> _pickVideo() async {
    final result = await FilePicker.pickFiles(
      type: FileType.video,
      withData: true,
    );
    final file = result?.files.single;
    if (file == null) {
      return;
    }
    setState(() {
      _selectedFile = file;
      if (_title.text.trim().isEmpty) {
        _title.text = file.name;
      }
      _message = null;
    });
  }

  Future<void> _uploadVideo() async {
    final session = widget.sessionController.session;
    if (session == null) {
      setState(() => _message = 'Sign in before uploading.');
      return;
    }
    final file = _selectedFile;
    final bytes = file?.bytes;
    if (file == null || bytes == null || bytes.isEmpty) {
      setState(() => _message = 'Choose a video file first.');
      return;
    }

    setState(() {
      _loading = true;
      _message = null;
    });
    try {
      final checksum = sha256.convert(bytes).toString();
      final contentType = _contentTypeFor(file.extension);
      final intent = await widget.repository.createUploadIntent(
        token: session.accessToken,
        title: _title.text.trim().isEmpty ? file.name : _title.text.trim(),
        description: _description.text.trim(),
        visibility: _visibility,
        contentType: contentType,
        sizeBytes: file.size,
        checksumSha256: checksum,
      );
      await widget.repository.uploadObject(
        uploadUrl: Uri.parse(intent.uploadUrl),
        bytes: bytes,
        contentType: contentType,
      );
      await widget.repository.confirmUploaded(
        token: session.accessToken,
        videoId: intent.videoId,
        uploadRequestId: intent.uploadRequestId,
        sizeBytes: file.size,
        checksumSha256: checksum,
      );
      setState(() {
        _selectedFile = null;
        _message = 'Uploaded ${intent.videoId}. Processing should start soon.';
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
          subtitle: 'Pick a video, upload it, then track processing.',
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
                initialValue: _visibility,
                decoration: const InputDecoration(labelText: 'Visibility'),
                items: const [
                  DropdownMenuItem(value: 'public', child: Text('Public')),
                  DropdownMenuItem(value: 'private', child: Text('Private')),
                ],
                onChanged: (value) => setState(() => _visibility = value ?? 'public'),
              ),
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: _loading ? null : _pickVideo,
                icon: const Icon(Icons.video_file_outlined),
                label: Text(_selectedFile == null ? 'Choose video' : _selectedFile!.name),
              ),
              if (_selectedFile != null) ...[
                const SizedBox(height: 8),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    '${_formatBytes(_selectedFile!.size)} · ${_contentTypeFor(_selectedFile!.extension)}',
                    style: const TextStyle(color: Color(0xFF6B7280)),
                  ),
                ),
              ],
              if (_message != null) ...[
                const SizedBox(height: 10),
                Text(_message!, style: const TextStyle(color: Color(0xFF374151))),
              ],
              const SizedBox(height: 14),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: _loading ? null : _uploadVideo,
                  icon: const Icon(Icons.cloud_upload_outlined),
                  label: Text(_loading ? 'Uploading...' : 'Upload video'),
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

  String _contentTypeFor(String? extension) {
    return switch ((extension ?? '').toLowerCase()) {
      'mov' => 'video/quicktime',
      'm4v' => 'video/x-m4v',
      'webm' => 'video/webm',
      _ => 'video/mp4',
    };
  }

  String _formatBytes(int value) {
    if (value < 1024) {
      return '$value B';
    }
    if (value < 1024 * 1024) {
      return '${(value / 1024).toStringAsFixed(1)} KB';
    }
    return '${(value / (1024 * 1024)).toStringAsFixed(1)} MB';
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
