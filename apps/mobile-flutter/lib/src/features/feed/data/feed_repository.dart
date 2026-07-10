import '../../../core/network/api_client.dart';
import '../domain/feed_item.dart';

class FeedRepository {
  const FeedRepository({required ApiClient apiClient}) : _apiClient = apiClient;

  final ApiClient _apiClient;

  Future<List<FeedItem>> listFeed({int limit = 20}) async {
    final body = await _apiClient.get('/api/v1/feed?limit=$limit');
    final items = body['data'] as List<dynamic>? ?? const [];
    return items
        .map((item) => FeedItem.fromJson(item as Map<String, dynamic>))
        .toList();
  }
}
