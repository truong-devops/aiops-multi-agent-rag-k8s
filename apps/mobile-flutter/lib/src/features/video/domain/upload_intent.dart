class UploadIntent {
  const UploadIntent({
    required this.videoId,
    required this.uploadRequestId,
    required this.uploadUrl,
    required this.objectKey,
    required this.expiresAt,
  });

  final String videoId;
  final String uploadRequestId;
  final String uploadUrl;
  final String objectKey;
  final String expiresAt;

  factory UploadIntent.fromJson(Map<String, dynamic> json) {
    final video = json['video'] as Map<String, dynamic>? ?? const {};
    final upload = json['upload_request'] as Map<String, dynamic>? ?? const {};
    return UploadIntent(
      videoId: video['id'] as String? ?? '',
      uploadRequestId: upload['id'] as String? ?? '',
      uploadUrl: upload['upload_url'] as String? ?? '',
      objectKey: upload['object_key'] as String? ?? '',
      expiresAt: upload['expires_at'] as String? ?? '',
    );
  }
}
