class FeedItem {
  const FeedItem({
    required this.videoId,
    required this.ownerId,
    required this.title,
    required this.description,
    required this.thumbnailObjectKey,
    required this.playbackObjectKey,
    required this.durationMs,
    required this.likeCount,
    required this.commentCount,
    required this.readyAt,
  });

  final String videoId;
  final String ownerId;
  final String title;
  final String description;
  final String thumbnailObjectKey;
  final String playbackObjectKey;
  final int durationMs;
  final int likeCount;
  final int commentCount;
  final String readyAt;

  factory FeedItem.fromJson(Map<String, dynamic> json) {
    final owner = json['owner'] as Map<String, dynamic>? ?? const {};
    return FeedItem(
      videoId: json['video_id'] as String? ?? '',
      ownerId: owner['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      description: json['description'] as String? ?? '',
      thumbnailObjectKey: json['thumbnail_object_key'] as String? ?? '',
      playbackObjectKey: json['playback_object_key'] as String? ?? '',
      durationMs: json['duration_ms'] as int? ?? 0,
      likeCount: json['like_count'] as int? ?? 0,
      commentCount: json['comment_count'] as int? ?? 0,
      readyAt: json['ready_at'] as String? ?? '',
    );
  }
}
