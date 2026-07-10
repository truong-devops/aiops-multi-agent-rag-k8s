import '../../../core/network/api_client.dart';
import '../domain/upload_intent.dart';
import '../domain/video_item.dart';

class VideoRepository {
  const VideoRepository({required ApiClient apiClient}) : _apiClient = apiClient;

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
}
