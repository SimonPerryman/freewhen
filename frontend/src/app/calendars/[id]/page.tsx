import type { Metadata } from "next";
import { getCalendar } from "@/lib/api";
import { titlePlain } from "@/lib/title";
import CalendarPageClient from "./CalendarPageClient";

interface Props {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;

  try {
    const data = await getCalendar(id);
    const rawName = data.calendar.name || "Untitled Calendar";
    const name = titlePlain(rawName);
    const participantCount = data.participants.length;
    const description =
      participantCount > 0
        ? `${participantCount} ${participantCount === 1 ? "person has" : "people have"} responded — mark your availability`
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
  return <CalendarPageClient calendarId={id} />;
}
