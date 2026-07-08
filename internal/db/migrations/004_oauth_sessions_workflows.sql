-- OAuth backchannel logout (sessions.oauthSid) and persisted workflows.

ALTER TABLE public.sessions
    ADD COLUMN IF NOT EXISTS "oauthSid" character varying;

CREATE INDEX IF NOT EXISTS "IDX_sessions_oauth_sid" ON public.sessions USING btree ("oauthSid");

CREATE TABLE IF NOT EXISTS public.workflows (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "ownerId" uuid NOT NULL,
    name character varying NOT NULL,
    description character varying DEFAULT ''::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    status character varying DEFAULT 'active'::character varying NOT NULL,
    "trigger" jsonb NOT NULL,
    actions jsonb NOT NULL,
    "executionCount" integer DEFAULT 0 NOT NULL,
    "lastExecutionAt" timestamp with time zone,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT workflows_pkey PRIMARY KEY (id),
    CONSTRAINT "workflows_ownerId_fkey" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_workflows_ownerId" ON public.workflows USING btree ("ownerId");

CREATE TABLE IF NOT EXISTS public.workflow_executions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "workflowId" uuid NOT NULL,
    status character varying DEFAULT 'pending'::character varying NOT NULL,
    "startedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "completedAt" timestamp with time zone,
    "errorMessage" character varying,
    "triggerData" jsonb,
    "actionResults" jsonb,
    CONSTRAINT workflow_executions_pkey PRIMARY KEY (id),
    CONSTRAINT "workflow_executions_workflowId_fkey" FOREIGN KEY ("workflowId") REFERENCES public.workflows(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_workflow_executions_workflowId" ON public.workflow_executions USING btree ("workflowId");
