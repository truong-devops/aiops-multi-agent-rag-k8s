import 'dart:typed_data';

import '../../../core/network/api_client.dart';
import '../domain/upload_intent.dart';
import '../domain/video_item.dart';

class VideoRepository {
  const VideoRepository({required ApiClient apiClient})
      : _apiClient = apiClient;

  final ApiClient _apiClient;

  Future<List<VideoItem>> listMyVideos(String token, {int limit = 20}) async {
    final body = await _apiClient.get(
      '/api/v1/videos?limit=$limit',
      token: token,
    );
    final items = body['data'] as List<dynamic>? ?? const [];
    return items
        .map((item) => VideoItem.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  Future<UploadIntent> createUploadIntent({
    required String token,
    required String title,
    required String description,
    required String visibility,
    required String contentType,
    required int sizeBytes,
    required String checksumSha256,
  }) async {
    final body = await _apiClient.post(
      '/api/v1/videos/upload-requests',
      token: token,
      idempotencyKey: DateTime.now().microsecondsSinceEpoch.toString(),
      body: {
        'title': title,
        'description': description,
        'visibility': visibility,
        'content_type': contentType,
        'size_bytes': sizeBytes,
        'checksum_sha256': checksumSha256,
      },
    );
    return UploadIntent.fromJson(body['data'] as Map<String, dynamic>);
  }

  Future<void> uploadObject({
    required Uri uploadUrl,
    required Uint8List bytes,
    required String contentType,
  }) {
    return _apiClient.putBytes(
      uploadUrl,
      bytes: bytes,
      contentType: contentType,
    );
  }

  Future<VideoItem> confirmUploaded({
    required String token,
    required String videoId,
    required String uploadRequestId,
    required int sizeBytes,
    required String checksumSha256,
  }) async {
    final body = await _apiClient.post(
      '/api/v1/videos/$videoId/uploaded',
      token: token,
      body: {
        'upload_request_id': uploadRequestId,
        'size_bytes': sizeBytes,
        'checksum_sha256': checksumSha256,
      },
    );
    final data = body['data'] as Map<String, dynamic>;
    return VideoItem.fromJson(data['video'] as Map<String, dynamic>);
  }
}
