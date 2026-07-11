import 'package:aiops_video_mobile/src/app.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders mobile app shell', (tester) async {
    await tester.pumpWidget(const AiopsVideoMobileApp());
    expect(find.text('AIOps Video'), findsOneWidget);
    expect(find.text('Feed'), findsWidgets);
  });
}
