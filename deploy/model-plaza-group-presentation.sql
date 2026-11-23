-- Checked against ranxi v2.24.2 991cd30 settings table
-- (backend/migrations/005_schema_parity.sql): id BIGSERIAL, key VARCHAR(100) UNIQUE,
-- value TEXT, updated_at TIMESTAMPTZ DEFAULT NOW(). ON CONFLICT (key) matches that unique key.
--
-- One-time, reversible presentation patch for this site.
-- Do not put this file in backend/migrations: it must not run on deploy by itself.
--
-- Effect: model-plaza shows group 55 (视频分组) immediately after group 53
-- (GPT特惠分组0.2). Every other group's relative order stays on the legacy
-- effective-rate then name sort. Group 55 therefore sits before the Grok group
-- when that group still sorts after group 53.
--
-- This statement writes only settings.key = 'model_plaza_display'.
-- It does not change groups.rate_multiplier, groups.sort_order, user rates,
-- channel prices, model-pricing overrides, or account schedulable flags.
-- Video reference rows are shipped in the application seed and do not need a
-- row here. Leaving video_reference_override unset keeps that seed.
--
-- Apply on the maintenance database, then reload the model plaza page.
-- Existing groups having sort_order = 0 is expected and is not used.

BEGIN;

INSERT INTO settings (key, value, updated_at)
VALUES (
  'model_plaza_display',
  '{"insert_after":[{"group_id":55,"after_group_id":53}]}',
  NOW()
)
ON CONFLICT (key) DO UPDATE
SET value = (
      jsonb_set(
        CASE
          WHEN settings.value ~ '^\s*\{' THEN settings.value::jsonb
          ELSE '{}'::jsonb
        END,
        '{insert_after}',
        '[{"group_id":55,"after_group_id":53}]'::jsonb,
        true
      )
    )::text,
    updated_at = NOW();

COMMIT;

-- Rollback (run separately, not as part of the apply transaction):
-- UPDATE settings
-- SET value = ((value::jsonb - 'insert_after'))::text,
--     updated_at = NOW()
-- WHERE key = 'model_plaza_display';
--
-- DELETE FROM settings
-- WHERE key = 'model_plaza_display'
--   AND COALESCE(NULLIF(value, '')::jsonb, '{}'::jsonb) = '{}'::jsonb;
