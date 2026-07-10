class VideoItem {
  const VideoItem({
    required this.id,
    required this.title,
    required this.status,
    required this.visibility,
    required this.updatedAt,
  });

  final String id;
  final String title;
  final String status;
  final String visibility;
  final String updatedAt;

  factory VideoItem.fromJson(Map<String, dynamic> json) {
    return VideoItem(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      status: json['status'] as String? ?? '',
      visibility: json['visibility'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}
