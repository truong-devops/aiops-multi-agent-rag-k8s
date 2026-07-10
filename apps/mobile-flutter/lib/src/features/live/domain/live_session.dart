class LiveSession {
  const LiveSession({
    required this.id,
    required this.title,
    required this.status,
    required this.playbackUrl,
    required this.updatedAt,
  });

  final String id;
  final String title;
  final String status;
  final String playbackUrl;
  final String updatedAt;

  factory LiveSession.fromJson(Map<String, dynamic> json) {
    return LiveSession(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      status: json['status'] as String? ?? '',
      playbackUrl: json['playback_url'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}
