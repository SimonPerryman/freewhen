-- Records that the user has seen the privacy interstitial before the first
-- Google Calendar import. Null → show the interstitial on next import attempt;
-- set → skip straight to OAuth. Doubles as our informed-consent record.
ALTER TABLE users ADD COLUMN calendar_import_consent_ack_at TIMESTAMPTZ;
