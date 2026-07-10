import 'package:flutter/material.dart';

enum StatusTone { ready, warning, danger, neutral }

class StatusChip extends StatelessWidget {
  const StatusChip({
    super.key,
    required this.label,
    required this.tone,
  });

  factory StatusChip.fromStatus(String status) {
    final normalized = status.toLowerCase();
    final tone = switch (normalized) {
      'ready' || 'live' || 'active' => StatusTone.ready,
      'pending' || 'uploaded' || 'processing' || 'scheduled' => StatusTone.warning,
      'failed' || 'ended' || 'disabled' => StatusTone.danger,
      _ => StatusTone.neutral,
    };
    return StatusChip(label: status.isEmpty ? 'unknown' : status, tone: tone);
  }

  final String label;
  final StatusTone tone;

  @override
  Widget build(BuildContext context) {
    final colors = switch (tone) {
      StatusTone.ready => (const Color(0xFFDCFCE7), const Color(0xFF166534)),
      StatusTone.warning => (const Color(0xFFFEF3C7), const Color(0xFF92400E)),
      StatusTone.danger => (const Color(0xFFFEE2E2), const Color(0xFF991B1B)),
      StatusTone.neutral => (const Color(0xFFF1F5F9), const Color(0xFF475569)),
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: colors.$1,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: colors.$2,
          fontSize: 12,
          fontWeight: FontWeight.w800,
        ),
      ),
    );
  }
}
