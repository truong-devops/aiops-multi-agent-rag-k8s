import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "AIOps Video Platform Admin",
  description: "Operations console for the video platform and AIOps demo"
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
