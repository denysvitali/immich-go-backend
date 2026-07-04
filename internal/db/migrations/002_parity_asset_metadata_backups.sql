-- Asset metadata, edits, OCR, database backups, and session sync reset state.

ALTER TABLE public.sessions
    ADD COLUMN IF NOT EXISTS "isPendingSyncReset" boolean DEFAULT false NOT NULL,
    ADD COLUMN IF NOT EXISTS "appVersion" character varying;

CREATE TABLE IF NOT EXISTS public.asset_metadata (
    "assetId" uuid NOT NULL,
    key character varying NOT NULL,
    value jsonb NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT asset_metadata_pkey PRIMARY KEY ("assetId", key),
    CONSTRAINT asset_metadata_asset_fkey FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.asset_edits (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "assetId" uuid NOT NULL,
    action character varying NOT NULL,
    parameters jsonb NOT NULL,
    position integer DEFAULT 0 NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT asset_edits_pkey PRIMARY KEY (id),
    CONSTRAINT asset_edits_asset_fkey FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.asset_ocr (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "assetId" uuid NOT NULL,
    text text NOT NULL,
    "textScore" double precision DEFAULT 0 NOT NULL,
    "boxScore" double precision DEFAULT 0 NOT NULL,
    x1 double precision DEFAULT 0 NOT NULL,
    y1 double precision DEFAULT 0 NOT NULL,
    x2 double precision DEFAULT 0 NOT NULL,
    y2 double precision DEFAULT 0 NOT NULL,
    x3 double precision DEFAULT 0 NOT NULL,
    y3 double precision DEFAULT 0 NOT NULL,
    x4 double precision DEFAULT 0 NOT NULL,
    y4 double precision DEFAULT 0 NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT asset_ocr_pkey PRIMARY KEY (id),
    CONSTRAINT asset_ocr_asset_fkey FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.database_backups (
    filename character varying NOT NULL,
    path character varying NOT NULL,
    filesize bigint DEFAULT 0 NOT NULL,
    timezone character varying DEFAULT 'UTC'::character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT database_backups_pkey PRIMARY KEY (filename)
);

CREATE INDEX IF NOT EXISTS asset_edits_asset_id_idx ON public.asset_edits ("assetId", position);
CREATE INDEX IF NOT EXISTS asset_ocr_asset_id_idx ON public.asset_ocr ("assetId");

DROP TRIGGER IF EXISTS asset_metadata_updated_at ON public.asset_metadata;
CREATE TRIGGER asset_metadata_updated_at BEFORE UPDATE ON public.asset_metadata
    FOR EACH ROW EXECUTE FUNCTION public.updated_at();

DROP TRIGGER IF EXISTS asset_edits_updated_at ON public.asset_edits;
CREATE TRIGGER asset_edits_updated_at BEFORE UPDATE ON public.asset_edits
    FOR EACH ROW EXECUTE FUNCTION public.updated_at();

DROP TRIGGER IF EXISTS asset_ocr_updated_at ON public.asset_ocr;
CREATE TRIGGER asset_ocr_updated_at BEFORE UPDATE ON public.asset_ocr
    FOR EACH ROW EXECUTE FUNCTION public.updated_at();

DROP TRIGGER IF EXISTS database_backups_updated_at ON public.database_backups;
CREATE TRIGGER database_backups_updated_at BEFORE UPDATE ON public.database_backups
    FOR EACH ROW EXECUTE FUNCTION public.updated_at();
