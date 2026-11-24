import type { Metadata } from "next";
import { headers } from "next/headers";
import { countResponded } from "@/lib/availabilityMaps";
import { localeFromAcceptLanguage } from "@/lib/dates";
import { DateLocaleProvider } from "@/lib/dateLocale";
import { getCalendarForServer, initialCalendarData, serverToday } from "@/lib/serverCalendar";
import { titlePlain } from "@/lib/title";
import CalendarPageClient from "./CalendarPageClient";

interface Props {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;

  try {
    // Same request-scoped fetch the page body uses (React cache()).
    const data = await getCalendarForServer(id);
    if (!data) throw new Error("calendar unavailable");
    const rawName = data.calendar.name || "Untitled Calendar";
    const name = titlePlain(rawName);
    const respondedCount = countResponded(data.participants, data.availability);
    const description =
      respondedCount > 0
        ? `${respondedCount} ${respondedCount === 1 ? "person has" : "people have"} responded — mark your availability`
        : "Mark your availability — no sign-up needed";

    // Cache-buster — bump whenever the calendar mutates so social previews
    // refresh. Falls back to createdAt if updatedAt is missing.
    const stamp = encodeURIComponent(data.calendar.updatedAt || data.calendar.createdAt || "");
    const ogUrl = `/calendars/${id}/opengraph-image${stamp ? `?v=${stamp}` : ""}`;
    const images = [{ url: ogUrl, width: 1200, height: 630, alt: name }];

    return {
      title: `${name} — FreeWhen`,
      description,
      openGraph: {
        title: name,
        description,
        siteName: "FreeWhen",
        type: "website",
        images,
      },
      twitter: {
        card: "summary_large_image",
        title: name,
        description,
        images,
      },
    };
  } catch {
    return {
      title: "FreeWhen",
      description: "Find when everyone's free. No sign-ups, no fuss.",
    };
  }
}

export default async function CalendarPage({ params }: Props) {
  const { id } = await params;
  // Only calendars an anonymous visitor can read are rendered on the server;
  // anything gated or failing falls back to the client fetch, which carries
  // the viewer's own credentials. See plans/ui/calendar-page-server-render.md.
  const initialData = initialCalendarData(await getCalendarForServer(id));
  // Dates render in the viewer's locale. The server only knows it from
  // Accept-Language; the client reuses the same value so hydration matches.
  const locale = localeFromAcceptLanguage((await headers()).get("accept-language"));
  return (
    <DateLocaleProvider locale={locale}>
      <CalendarPageClient
        key={id}
        calendarId={id}
        initialData={initialData}
        initialToday={initialData ? serverToday() : null}
      />
    </DateLocaleProvider>
  );
}
