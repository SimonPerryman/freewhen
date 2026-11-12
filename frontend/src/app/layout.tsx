import type { Metadata, Viewport } from "next";
import { DM_Sans, Fraunces, Instrument_Serif } from "next/font/google";
import { SpeedInsights } from "@vercel/speed-insights/next";
import AnalyticsProvider from "@/components/AnalyticsProvider/AnalyticsProvider";
import AuthProvider from "@/components/AuthProvider/AuthProvider";
import Header from "@/components/Header/Header";
import { COLOR_SCHEME_INIT_SCRIPT } from "@/lib/colorScheme";
import { ToastProvider } from "@/lib/toast";
import "./globals.css";

const body = DM_Sans({
  variable: "--font-body",
  subsets: ["latin"],
  display: "swap",
});

// The opsz axis is kept on purpose: h1–h3 use `font-optical-sizing: auto`,
// and Fraunces' optical sizes visibly change contrast/spacing at hero sizes.
const display = Fraunces({
  variable: "--font-display",
  subsets: ["latin"],
  axes: ["opsz"],
  display: "swap",
});

// The accent face is only ever rendered italic (`.serif-accent`, ShareButton
// kicker, coachmark title) on a word or two, so skip the normal style and
// don't preload it — it shouldn't compete with the body/display fonts for
// bandwidth on the critical path. `display: swap` covers the late arrival.
const accent = Instrument_Serif({
  variable: "--font-accent",
  subsets: ["latin"],
  weight: ["400"],
  style: ["italic"],
  display: "swap",
  preload: false,
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f3eee4" },
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
    <html
      lang="en"
      className={`${body.variable} ${display.variable} ${accent.variable}`}
      suppressHydrationWarning
    >
      <body>
        <script
          dangerouslySetInnerHTML={{
            __html: COLOR_SCHEME_INIT_SCRIPT,
          }}
        />
        <a href="#main" className="skip-link">
          Skip to content
        </a>
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
