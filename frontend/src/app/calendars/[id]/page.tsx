import type { Metadata } from "next";
import { getCalendar } from "@/lib/api";
import CalendarPageClient from "./CalendarPageClient";

interface Props {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;

  try {
    const data = await getCalendar(id);
    const name = data.calendar.name || "Untitled Calendar";
    const participantCount = data.participants.length;
    const description =
      participantCount > 0
        ? `${participantCount} ${participantCount === 1 ? "person has" : "people have"} responded — mark your availability`
        : "Mark your availability — no sign-up needed";

    return {
      title: `${name} — FreeWhen`,
      description,
      openGraph: {
        title: name,
        description,
        siteName: "FreeWhen",
        type: "website",
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
