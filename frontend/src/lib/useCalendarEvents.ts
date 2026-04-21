"use client";

import { useEffect, useRef, useState } from "react";
import {
  createSseStream,
  type RealtimeEvent,
  type RealtimeStream,
  type StreamFactory,
} from "./realtimeStream";
import analytics from "./analytics";
import { getCalendarAccessToken } from "./calendarAccess";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type StreamStatus = "connecting" | "open" | "polling" | "closed";

const REFRESH_DEBOUNCE_MS = 150;
const POLLING_INTERVAL_MS = 15000;
const MAX_CONSECUTIVE_ERRORS = 3;
const EVENT_ANALYTICS_SAMPLE_RATE = 0.1;

export interface UseCalendarEventsOptions {
  streamFactory?: StreamFactory;
}

export interface UseCalendarEventsResult {
  status: StreamStatus;
}

/**
 * Opens a live stream for the given calendar and calls `onRefresh` whenever
 * a server-side change arrives. Refreshes are debounced, so a burst of
 * events coalesces into a single refetch.
 *
 * Falls back to 15s polling if the stream fails to connect repeatedly.
 */
export function useCalendarEvents(
  calendarId: string,
  participantId: string | null,
  onRefresh: () => void,
  options: UseCalendarEventsOptions = {},
): UseCalendarEventsResult {
  const [status, setStatus] = useState<StreamStatus>("connecting");
  // Keep onRefresh in a ref so effect doesn't tear down on every render.
  const onRefreshRef = useRef(onRefresh);
  onRefreshRef.current = onRefresh;

  const factory = options.streamFactory ?? createSseStream;

  useEffect(() => {
    if (!calendarId) return;

    let debounceTimer: ReturnType<typeof setTimeout> | null = null;
    let pollingTimer: ReturnType<typeof setInterval> | null = null;
    let stream: RealtimeStream | null = null;
    let consecutiveErrors = 0;
    let fallbackAnnounced = false;
    let analyticsConnectedReported = false;

    const scheduleRefresh = () => {
      if (debounceTimer !== null) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        debounceTimer = null;
        onRefreshRef.current();
      }, REFRESH_DEBOUNCE_MS);
    };

    const startPolling = () => {
      if (pollingTimer !== null) return;
      setStatus("polling");
      if (!fallbackAnnounced) {
        fallbackAnnounced = true;
        analytics.capture("sse_fallback_polling", { calendar_id: calendarId });
      }
      pollingTimer = setInterval(() => onRefreshRef.current(), POLLING_INTERVAL_MS);
    };

    const params = new URLSearchParams();
    if (participantId) params.set("as", participantId);
    // EventSource can't set custom headers — pass the calendar access token
    // as a query param so gated calendars can still stream live updates.
    const accessToken = getCalendarAccessToken(calendarId);
    if (accessToken) params.set("access", accessToken);
    const query = params.toString() ? `?${params.toString()}` : "";
    const url = `${BASE_URL}/calendars/${calendarId}/events${query}`;

    try {
      stream = factory(url);
    } catch {
      startPolling();
      return () => {
        if (pollingTimer !== null) clearInterval(pollingTimer);
      };
    }

    stream.onStatus((s) => {
      if (s === "open") {
        consecutiveErrors = 0;
        if (pollingTimer !== null) {
          clearInterval(pollingTimer);
          pollingTimer = null;
        }
        setStatus("open");
        if (!analyticsConnectedReported) {
          analyticsConnectedReported = true;
          analytics.capture("sse_connected", { calendar_id: calendarId });
        }
        // Catch anything we missed before the stream was live.
        scheduleRefresh();
      } else if (s === "error") {
        consecutiveErrors += 1;
        if (consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
          stream?.close();
          stream = null;
          startPolling();
        }
      } else if (s === "connecting") {
        setStatus("connecting");
      }
    });

    stream.onEvent((e: RealtimeEvent) => {
      if (Math.random() < EVENT_ANALYTICS_SAMPLE_RATE) {
        analytics.capture("sse_event_received", {
          calendar_id: calendarId,
          type: e.type,
        });
      }
      scheduleRefresh();
    });

    return () => {
      if (debounceTimer !== null) clearTimeout(debounceTimer);
      if (pollingTimer !== null) clearInterval(pollingTimer);
      stream?.close();
      stream = null;
      setStatus("closed");
    };
  }, [calendarId, participantId, factory]);

  return { status };
}
