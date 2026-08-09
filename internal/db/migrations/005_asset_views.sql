-- Per-user asset view state used by the asset viewer endpoints.

CREATE TABLE IF NOT EXISTS public.asset_views (
    asset_id uuid NOT NULL,
    user_id uuid NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT asset_views_pkey PRIMARY KEY (asset_id, user_id),
    CONSTRAINT asset_views_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(id) ON DELETE CASCADE,
    CONSTRAINT asset_views_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_asset_views_asset_id ON public.asset_views USING btree (asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_views_user_id ON public.asset_views USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_asset_views_viewed_at ON public.asset_views USING btree (viewed_at DESC);
