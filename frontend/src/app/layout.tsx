import type { Metadata, Viewport } from "next";
import { DM_Sans, Bricolage_Grotesque } from "next/font/google";
import { SpeedInsights } from "@vercel/speed-insights/next";
import AnalyticsProvider from "@/components/AnalyticsProvider/AnalyticsProvider";
import AuthProvider from "@/components/AuthProvider/AuthProvider";
import Header from "@/components/Header/Header";
import { ToastProvider } from "@/lib/toast";
import "./globals.css";

const body = DM_Sans({
  variable: "--font-body",
  subsets: ["latin"],
});

const display = Bricolage_Grotesque({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["400", "600", "700"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#e06449" },
    { media: "(prefers-color-scheme: dark)", color: "#141210" },
  ],
};

export const metadata: Metadata = {
  title: "FreeWhen",
  description: "Find when everyone's free.",
  applicationName: "FreeWhen",
  appleWebApp: {
    capable: true,
    title: "FreeWhen",
    statusBarStyle: "default",
  },
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/favicon-32.png", type: "image/png", sizes: "32x32" },
      { url: "/icon.svg", type: "image/svg+xml" },
    ],
    apple: "/apple-touch-icon.png",
  },
  openGraph: {
    title: "FreeWhen",
    description: "Find when everyone's free.",
    siteName: "FreeWhen",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${body.variable} ${display.variable}`}>
      <body>
        <AnalyticsProvider>
          <AuthProvider>
            <ToastProvider>
              <Header />
              {children}
            </ToastProvider>
          </AuthProvider>
        </AnalyticsProvider>
        <SpeedInsights />
      </body>
    </html>
  );
}
