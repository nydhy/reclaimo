import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Reclaimo",
  description: "Email-first autonomous price recovery agent",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}

