-- Dead-letter table for jobs that exhausted their retry policy or were marked
-- non-retryable (e.g. unsupported job types).

CREATE TABLE IF NOT EXISTS public.job_failures (
    id uuid DEFAULT public.gen_random_uuid() NOT NULL,
    queue text NOT NULL,
    job_type text NOT NULL,
    payload jsonb NOT NULL,
    error text NOT NULL,
    max_retries integer NOT NULL,
    retried_count integer NOT NULL DEFAULT 0,
    failed_at timestamp with time zone DEFAULT now() NOT NULL,
    last_failed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT job_failures_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS job_failures_failed_at_idx ON public.job_failures (failed_at DESC);
CREATE INDEX IF NOT EXISTS job_failures_job_type_idx ON public.job_failures (job_type);
