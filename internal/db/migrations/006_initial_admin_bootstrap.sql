-- Existing installations already completed initial setup before the durable
-- bootstrap claim was introduced. Backfill the singleton marker so deleting
-- every user cannot reopen the unauthenticated admin-sign-up endpoint.
INSERT INTO public.system_metadata (key, value)
SELECT
    'initial_admin_bootstrap',
    jsonb_build_object(
        'initializedAt', now(),
        'source', 'existing_users'
    )
WHERE EXISTS (SELECT 1 FROM public.users)
ON CONFLICT (key) DO NOTHING;
