import "./globals.css";

export const metadata = {
  title: "YT Downloader",
  description: "Mobile-first YouTube downloader utility (MVP scaffold)"
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
