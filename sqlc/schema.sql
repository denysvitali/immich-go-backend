CREATE FUNCTION public.immich_uuid_v7(p_timestamp timestamp with time zone DEFAULT clock_timestamp()) RETURNS uuid
    LANGUAGE sql
    AS $$
select encode(
               set_bit(
                       set_bit(
                               overlay(uuid_send(gen_random_uuid())
                                   placing substring(int8send(floor(extract(epoch from p_timestamp) * 1000)::bigint) from 3)
                            from 1 for 6
                               ),
                               52, 1
                       ),
                       53, 1
               ),
               'hex')::uuid;
$$;

CREATE TYPE public.asset_visibility_enum AS ENUM (
    'archive',
    'timeline',
    'hidden',
    'locked'
);


--
-- Name: assets_status_enum; Type: TYPE; Schema: public; Owner: immich
--

CREATE TYPE public.assets_status_enum AS ENUM (
    'active',
    'trashed',
    'deleted'
);


--
-- Name: sourcetype; Type: TYPE; Schema: public; Owner: immich
--

CREATE TYPE public.sourcetype AS ENUM (
    'machine-learning',
    'exif',
    'manual'
);


--
-- Name: album_user_after_insert(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.album_user_after_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      UPDATE albums SET "updatedAt" = clock_timestamp(), "updateId" = immich_uuid_v7(clock_timestamp())
      WHERE "id" IN (SELECT DISTINCT "albumsId" FROM inserted_rows);
      RETURN NULL;
    END
  $$;


--
-- Name: album_users_delete_audit(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.album_users_delete_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO albums_audit ("albumId", "userId")
      SELECT "albumsId", "usersId"
      FROM OLD;

      IF pg_trigger_depth() = 1 THEN
        INSERT INTO album_users_audit ("albumId", "userId")
        SELECT "albumsId", "usersId"
        FROM OLD;
      END IF;

      RETURN NULL;
    END
  $$;


--
-- Name: albums_delete_audit(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.albums_delete_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
      INSERT INTO albums_audit ("albumId", "userId")
      SELECT "id", "ownerId"
      FROM OLD;
      RETURN NULL;
    END
  $$;


--
-- Name: assets_delete_audit(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.assets_delete_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
           BEGIN
            INSERT INTO assets_audit ("assetId", "ownerId")
            SELECT "id", "ownerId"
            FROM OLD;
            RETURN NULL;
           END;
          $$;


--
-- Name: f_concat_ws(text, text[]); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.f_concat_ws(text, text[]) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $_$SELECT array_to_string($2, $1)$_$;


--
-- Name: f_unaccent(text); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.f_unaccent(text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN public.unaccent('public.unaccent'::regdictionary, $1);


--
-- Name: immich_uuid_v7(timestamp with time zone); Type: FUNCTION; Schema: public; Owner: immich
--




--
-- Name: ll_to_earth_public(double precision, double precision); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.ll_to_earth_public(latitude double precision, longitude double precision) RETURNS public.earth
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    AS $$
        SELECT public.cube(public.cube(public.cube(public.earth()*cos(radians(latitude))*cos(radians(longitude))),public.earth()*cos(radians(latitude))*sin(radians(longitude))),public.earth()*sin(radians(latitude)))::public.earth
    $$;


--
-- Name: partners_delete_audit(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.partners_delete_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
               BEGIN
                INSERT INTO partners_audit ("sharedById", "sharedWithId")
                SELECT "sharedById", "sharedWithId"
                FROM OLD;
                RETURN NULL;
               END;
              $$;


--
-- Name: updated_at(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
          DECLARE
              clock_timestamp TIMESTAMP := clock_timestamp();
          BEGIN
              new."updatedAt" = clock_timestamp;
              new."updateId" = immich_uuid_v7(clock_timestamp);
              return new;
          END;
          $$;


--
-- Name: user_search(text); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.user_search(uname text) RETURNS TABLE(usename name, passwd text)
    LANGUAGE sql SECURITY DEFINER
    AS $_$SELECT usename, passwd FROM pg_catalog.pg_shadow WHERE usename=$1;$_$;


--
-- Name: users_delete_audit(); Type: FUNCTION; Schema: public; Owner: immich
--

CREATE FUNCTION public.users_delete_audit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
           BEGIN
            INSERT INTO users_audit ("userId")
            SELECT "id"
            FROM OLD;
            RETURN NULL;
           END;
          $$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: activity; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.activity (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "albumId" uuid NOT NULL,
    "userId" uuid NOT NULL,
    "assetId" uuid,
    comment text,
    "isLiked" boolean DEFAULT false NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    CONSTRAINT "CHK_2ab1e70f113f450eb40c1e3ec8" CHECK ((((comment IS NULL) AND ("isLiked" = true)) OR ((comment IS NOT NULL) AND ("isLiked" = false))))
);


--
-- Name: album_users_audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.album_users_audit (
    id uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "albumId" uuid NOT NULL,
    "userId" uuid NOT NULL,
    "deletedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: albums; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.albums (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "ownerId" uuid NOT NULL,
    "albumName" character varying DEFAULT 'Untitled Album'::character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "albumThumbnailAssetId" uuid,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    "deletedAt" timestamp with time zone,
    "isActivityEnabled" boolean DEFAULT true NOT NULL,
    "order" character varying DEFAULT 'desc'::character varying NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: COLUMN albums."albumThumbnailAssetId"; Type: COMMENT; Schema: public; Owner: immich
--

COMMENT ON COLUMN public.albums."albumThumbnailAssetId" IS 'Asset ID to be used as thumbnail';


--
-- Name: albums_assets_assets; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.albums_assets_assets (
    "albumsId" uuid NOT NULL,
    "assetsId" uuid NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: albums_audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.albums_audit (
    id uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "albumId" uuid NOT NULL,
    "userId" uuid NOT NULL,
    "deletedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: albums_shared_users_users; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.albums_shared_users_users (
    "albumsId" uuid NOT NULL,
    "usersId" uuid NOT NULL,
    role character varying DEFAULT 'editor'::character varying NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: api_keys; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.api_keys (
    name character varying NOT NULL,
    key character varying NOT NULL,
    "userId" uuid NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    permissions character varying[] NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: asset_faces; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.asset_faces (
    "assetId" uuid NOT NULL,
    "personId" uuid,
    "imageWidth" integer DEFAULT 0 NOT NULL,
    "imageHeight" integer DEFAULT 0 NOT NULL,
    "boundingBoxX1" integer DEFAULT 0 NOT NULL,
    "boundingBoxY1" integer DEFAULT 0 NOT NULL,
    "boundingBoxX2" integer DEFAULT 0 NOT NULL,
    "boundingBoxY2" integer DEFAULT 0 NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "sourceType" public.sourcetype DEFAULT 'machine-learning'::public.sourcetype NOT NULL,
    "deletedAt" timestamp with time zone
);


--
-- Name: asset_files; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.asset_files (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "assetId" uuid NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    type character varying NOT NULL,
    path character varying NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: asset_job_status; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.asset_job_status (
    "assetId" uuid NOT NULL,
    "facesRecognizedAt" timestamp with time zone,
    "metadataExtractedAt" timestamp with time zone,
    "duplicatesDetectedAt" timestamp with time zone,
    "previewAt" timestamp with time zone,
    "thumbnailAt" timestamp with time zone
);


--
-- Name: asset_stack; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.asset_stack (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "primaryAssetId" uuid NOT NULL,
    "ownerId" uuid NOT NULL
);


--
-- Name: assets; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.assets (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "deviceAssetId" character varying NOT NULL,
    "ownerId" uuid NOT NULL,
    "deviceId" character varying NOT NULL,
    type character varying NOT NULL,
    "originalPath" character varying NOT NULL,
    "fileCreatedAt" timestamp with time zone NOT NULL,
    "fileModifiedAt" timestamp with time zone NOT NULL,
    "isFavorite" boolean DEFAULT false NOT NULL,
    duration character varying,
    "encodedVideoPath" character varying DEFAULT ''::character varying,
    checksum bytea NOT NULL,
    "livePhotoVideoId" uuid,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "originalFileName" character varying NOT NULL,
    "sidecarPath" character varying,
    thumbhash bytea,
    "isOffline" boolean DEFAULT false NOT NULL,
    "libraryId" uuid,
    "isExternal" boolean DEFAULT false NOT NULL,
    "deletedAt" timestamp with time zone,
    "localDateTime" timestamp with time zone NOT NULL,
    "stackId" uuid,
    "duplicateId" uuid,
    status public.assets_status_enum DEFAULT 'active'::public.assets_status_enum NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    visibility public.asset_visibility_enum DEFAULT 'timeline'::public.asset_visibility_enum NOT NULL
);


--
-- Name: assets_audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.assets_audit (
    id uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "assetId" uuid NOT NULL,
    "ownerId" uuid NOT NULL,
    "deletedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.audit (
    id integer NOT NULL,
    "entityType" character varying NOT NULL,
    "entityId" uuid NOT NULL,
    action character varying NOT NULL,
    "ownerId" uuid NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: audit_id_seq; Type: SEQUENCE; Schema: public; Owner: immich
--

CREATE SEQUENCE public.audit_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: immich
--


--
-- Name: exif; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.exif (
    "assetId" uuid NOT NULL,
    make character varying,
    model character varying,
    "exifImageWidth" integer,
    "exifImageHeight" integer,
    "fileSizeInByte" bigint,
    orientation character varying,
    "dateTimeOriginal" timestamp with time zone,
    "modifyDate" timestamp with time zone,
    "lensModel" character varying,
    "fNumber" double precision,
    "focalLength" double precision,
    iso integer,
    latitude double precision,
    longitude double precision,
    city character varying,
    state character varying,
    country character varying,
    description text DEFAULT ''::text NOT NULL,
    fps double precision,
    "exposureTime" character varying,
    "livePhotoCID" character varying,
    "timeZone" character varying,
    "projectionType" character varying,
    "profileDescription" character varying,
    colorspace character varying,
    "bitsPerSample" integer,
    "autoStackId" character varying,
    rating integer,
    "updatedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: face_search; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.face_search (
    "faceId" uuid NOT NULL,
    embedding public.vector(512) NOT NULL
);


--
-- Name: geodata_places; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.geodata_places (
    id integer NOT NULL,
    name character varying(200) NOT NULL,
    longitude double precision NOT NULL,
    latitude double precision NOT NULL,
    "countryCode" character(2) NOT NULL,
    "admin1Code" character varying(20),
    "admin2Code" character varying(80),
    "modificationDate" date NOT NULL,
    "admin1Name" character varying,
    "admin2Name" character varying,
    "alternateNames" character varying
);


--
-- Name: kysely_migrations; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.kysely_migrations (
    name character varying(255) NOT NULL,
    "timestamp" character varying(255) NOT NULL
);


--
-- Name: kysely_migrations_lock; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.kysely_migrations_lock (
    id character varying(255) NOT NULL,
    is_locked integer DEFAULT 0 NOT NULL
);


--
-- Name: libraries; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.libraries (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying NOT NULL,
    "ownerId" uuid NOT NULL,
    "importPaths" text[] NOT NULL,
    "exclusionPatterns" text[] NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "deletedAt" timestamp with time zone,
    "refreshedAt" timestamp with time zone,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: memories; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.memories (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "deletedAt" timestamp with time zone,
    "ownerId" uuid NOT NULL,
    type character varying NOT NULL,
    data jsonb NOT NULL,
    "isSaved" boolean DEFAULT false NOT NULL,
    "memoryAt" timestamp with time zone NOT NULL,
    "seenAt" timestamp with time zone,
    "showAt" timestamp with time zone,
    "hideAt" timestamp with time zone,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: memories_assets_assets; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.memories_assets_assets (
    "memoriesId" uuid NOT NULL,
    "assetsId" uuid NOT NULL
);


--
-- Name: migrations; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.migrations (
    id integer NOT NULL,
    "timestamp" bigint NOT NULL,
    name character varying NOT NULL
);


--
-- Name: migrations_id_seq; Type: SEQUENCE; Schema: public; Owner: immich
--

CREATE SEQUENCE public.migrations_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: migrations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: immich
--


--
-- Name: move_history; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.move_history (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "entityId" uuid NOT NULL,
    "pathType" character varying NOT NULL,
    "oldPath" character varying NOT NULL,
    "newPath" character varying NOT NULL
);


--
-- Name: naturalearth_countries; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.naturalearth_countries (
    id integer NOT NULL,
    admin character varying(50) NOT NULL,
    admin_a3 character varying(3) NOT NULL,
    type character varying(50) NOT NULL,
    coordinates polygon NOT NULL
);


--
-- Name: naturalearth_countries_tmp_id_seq; Type: SEQUENCE; Schema: public; Owner: immich
--

ALTER TABLE public.naturalearth_countries ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.naturalearth_countries_tmp_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    MAXVALUE 2147483647
    CACHE 1
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.notifications (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "deletedAt" timestamp with time zone,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "userId" uuid,
    level character varying DEFAULT 'info'::character varying NOT NULL,
    type character varying DEFAULT 'info'::character varying NOT NULL,
    data jsonb,
    title character varying NOT NULL,
    description text,
    "readAt" timestamp with time zone
);


--
-- Name: partners; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.partners (
    "sharedById" uuid NOT NULL,
    "sharedWithId" uuid NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "inTimeline" boolean DEFAULT false NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: partners_audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.partners_audit (
    id uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "sharedById" uuid NOT NULL,
    "sharedWithId" uuid NOT NULL,
    "deletedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL
);


--
-- Name: person; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.person (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "ownerId" uuid NOT NULL,
    name character varying DEFAULT ''::character varying NOT NULL,
    "thumbnailPath" character varying DEFAULT ''::character varying NOT NULL,
    "isHidden" boolean DEFAULT false NOT NULL,
    "birthDate" date,
    "faceAssetId" uuid,
    "isFavorite" boolean DEFAULT false NOT NULL,
    color character varying,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    CONSTRAINT "CHK_b0f82b0ed662bfc24fbb58bb45" CHECK (("birthDate" <= CURRENT_DATE))
);


--
-- Name: session_sync_checkpoints; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.session_sync_checkpoints (
    "sessionId" uuid NOT NULL,
    type character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    ack character varying NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.sessions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    token character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "userId" uuid NOT NULL,
    "deviceType" character varying DEFAULT ''::character varying NOT NULL,
    "deviceOS" character varying DEFAULT ''::character varying NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "pinExpiresAt" timestamp with time zone,
    "expiresAt" timestamp with time zone,
    "parentId" uuid
);


--
-- Name: shared_link__asset; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.shared_link__asset (
    "assetsId" uuid NOT NULL,
    "sharedLinksId" uuid NOT NULL
);


--
-- Name: shared_links; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.shared_links (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    description character varying,
    "userId" uuid NOT NULL,
    key bytea NOT NULL,
    type character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "expiresAt" timestamp with time zone,
    "allowUpload" boolean DEFAULT false NOT NULL,
    "albumId" uuid,
    "allowDownload" boolean DEFAULT true NOT NULL,
    "showExif" boolean DEFAULT true NOT NULL,
    password character varying
);


--
-- Name: smart_search; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.smart_search (
    "assetId" uuid NOT NULL,
    embedding public.vector(512) NOT NULL
);


--
-- Name: system_metadata; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.system_metadata (
    key character varying NOT NULL,
    value jsonb NOT NULL
);


--
-- Name: tag_asset; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.tag_asset (
    "assetsId" uuid NOT NULL,
    "tagsId" uuid NOT NULL
);


--
-- Name: tags; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.tags (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "userId" uuid NOT NULL,
    value character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    color character varying,
    "parentId" uuid,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: tags_closure; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.tags_closure (
    id_ancestor uuid NOT NULL,
    id_descendant uuid NOT NULL
);


--
-- Name: user_metadata; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.user_metadata (
    "userId" uuid NOT NULL,
    key character varying NOT NULL,
    value jsonb NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying NOT NULL,
    password character varying DEFAULT ''::character varying NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    "profileImagePath" character varying DEFAULT ''::character varying NOT NULL,
    "isAdmin" boolean DEFAULT false NOT NULL,
    "shouldChangePassword" boolean DEFAULT true NOT NULL,
    "deletedAt" timestamp with time zone,
    "oauthId" character varying DEFAULT ''::character varying NOT NULL,
    "updatedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "storageLabel" character varying,
    name character varying DEFAULT ''::character varying NOT NULL,
    "quotaSizeInBytes" bigint,
    "quotaUsageInBytes" bigint DEFAULT '0'::bigint NOT NULL,
    status character varying DEFAULT 'active'::character varying NOT NULL,
    "profileChangedAt" timestamp with time zone DEFAULT now() NOT NULL,
    "updateId" uuid DEFAULT public.immich_uuid_v7() NOT NULL,
    "avatarColor" character varying,
    "pinCode" character varying
);


--
-- Name: users_audit; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.users_audit (
    "userId" uuid NOT NULL,
    "deletedAt" timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    id uuid DEFAULT public.immich_uuid_v7() NOT NULL
);


--
-- Name: version_history; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.version_history (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    "createdAt" timestamp with time zone DEFAULT now() NOT NULL,
    version character varying NOT NULL
);


--
-- Name: audit id; Type: DEFAULT; Schema: public; Owner: immich
--


--
-- Name: migrations id; Type: DEFAULT; Schema: public; Owner: immich
--


--
-- Name: audit PK_1d3d120ddaf7bc9b1ed68ed463a; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.audit
    ADD CONSTRAINT "PK_1d3d120ddaf7bc9b1ed68ed463a" PRIMARY KEY (id);


--
-- Name: activity PK_24625a1d6b1b089c8ae206fe467; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT "PK_24625a1d6b1b089c8ae206fe467" PRIMARY KEY (id);


--
-- Name: asset_job_status PK_420bec36fc02813bddf5c8b73d4; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_job_status
    ADD CONSTRAINT "PK_420bec36fc02813bddf5c8b73d4" PRIMARY KEY ("assetId");


--
-- Name: sessions PK_48cb6b5c20faa63157b3c1baf7f; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT "PK_48cb6b5c20faa63157b3c1baf7f" PRIMARY KEY (id);


--
-- Name: libraries PK_505fedfcad00a09b3734b4223de; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.libraries
    ADD CONSTRAINT "PK_505fedfcad00a09b3734b4223de" PRIMARY KEY (id);


--
-- Name: user_metadata PK_5931462150b3438cbc83277fe5a; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.user_metadata
    ADD CONSTRAINT "PK_5931462150b3438cbc83277fe5a" PRIMARY KEY ("userId", key);


--
-- Name: api_keys PK_5c8a79801b44bd27b79228e1dad; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT "PK_5c8a79801b44bd27b79228e1dad" PRIMARY KEY (id);


--
-- Name: version_history PK_5db259cbb09ce82c0d13cfd1b23; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.version_history
    ADD CONSTRAINT "PK_5db259cbb09ce82c0d13cfd1b23" PRIMARY KEY (id);


--
-- Name: person PK_5fdaf670315c4b7e70cce85daa3; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.person
    ADD CONSTRAINT "PK_5fdaf670315c4b7e70cce85daa3" PRIMARY KEY (id);


--
-- Name: shared_links PK_642e2b0f619e4876e5f90a43465; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_links
    ADD CONSTRAINT "PK_642e2b0f619e4876e5f90a43465" PRIMARY KEY (id);


--
-- Name: notifications PK_6a72c3c0f683f6462415e653c3a; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT "PK_6a72c3c0f683f6462415e653c3a" PRIMARY KEY (id);


--
-- Name: asset_faces PK_6df76ab2eb6f5b57b7c2f1fc684; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_faces
    ADD CONSTRAINT "PK_6df76ab2eb6f5b57b7c2f1fc684" PRIMARY KEY (id);


--
-- Name: asset_stack PK_74a27e7fcbd5852463d0af3034b; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_stack
    ADD CONSTRAINT "PK_74a27e7fcbd5852463d0af3034b" PRIMARY KEY (id);


--
-- Name: albums_shared_users_users PK_7df55657e0b2e8b626330a0ebc8; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_shared_users_users
    ADD CONSTRAINT "PK_7df55657e0b2e8b626330a0ebc8" PRIMARY KEY ("albumsId", "usersId");


--
-- Name: albums PK_7f71c7b5bc7c87b8f94c9a93a00; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums
    ADD CONSTRAINT "PK_7f71c7b5bc7c87b8f94c9a93a00" PRIMARY KEY (id);


--
-- Name: migrations PK_8c82d7f526340ab734260ea46be; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.migrations
    ADD CONSTRAINT "PK_8c82d7f526340ab734260ea46be" PRIMARY KEY (id);


--
-- Name: partners_audit PK_952b50217ff78198a7e380f0359; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.partners_audit
    ADD CONSTRAINT "PK_952b50217ff78198a7e380f0359" PRIMARY KEY (id);


--
-- Name: assets_audit PK_99bd5c015f81a641927a32b4212; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets_audit
    ADD CONSTRAINT "PK_99bd5c015f81a641927a32b4212" PRIMARY KEY (id);


--
-- Name: shared_link__asset PK_9b4f3687f9b31d1e311336b05e3; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_link__asset
    ADD CONSTRAINT "PK_9b4f3687f9b31d1e311336b05e3" PRIMARY KEY ("assetsId", "sharedLinksId");


--
-- Name: users PK_a3ffb1c0c8416b9fc6f907b7433; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "PK_a3ffb1c0c8416b9fc6f907b7433" PRIMARY KEY (id);


--
-- Name: memories PK_aaa0692d9496fe827b0568612f8; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.memories
    ADD CONSTRAINT "PK_aaa0692d9496fe827b0568612f8" PRIMARY KEY (id);


--
-- Name: move_history PK_af608f132233acf123f2949678d; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.move_history
    ADD CONSTRAINT "PK_af608f132233acf123f2949678d" PRIMARY KEY (id);


--
-- Name: session_sync_checkpoints PK_b846ab547a702863ef7cd9412fb; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.session_sync_checkpoints
    ADD CONSTRAINT "PK_b846ab547a702863ef7cd9412fb" PRIMARY KEY ("sessionId", type);


--
-- Name: exif PK_c0117fdbc50b917ef9067740c44; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.exif
    ADD CONSTRAINT "PK_c0117fdbc50b917ef9067740c44" PRIMARY KEY ("assetId");


--
-- Name: asset_files PK_c41dc3e9ef5e1c57ca5a08a0004; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_files
    ADD CONSTRAINT "PK_c41dc3e9ef5e1c57ca5a08a0004" PRIMARY KEY (id);


--
-- Name: albums_assets_assets PK_c67bc36fa845fb7b18e0e398180; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_assets_assets
    ADD CONSTRAINT "PK_c67bc36fa845fb7b18e0e398180" PRIMARY KEY ("albumsId", "assetsId");


--
-- Name: albums_audit PK_c75efea8d4dce316ad29b851a8b; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_audit
    ADD CONSTRAINT "PK_c75efea8d4dce316ad29b851a8b" PRIMARY KEY (id);


--
-- Name: assets PK_da96729a8b113377cfb6a62439c; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT "PK_da96729a8b113377cfb6a62439c" PRIMARY KEY (id);


--
-- Name: tags PK_e7dc17249a1148a1970748eda99; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT "PK_e7dc17249a1148a1970748eda99" PRIMARY KEY (id);


--
-- Name: users_audit PK_e9b2bdfd90e7eb5961091175180; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.users_audit
    ADD CONSTRAINT "PK_e9b2bdfd90e7eb5961091175180" PRIMARY KEY (id);


--
-- Name: tags_closure PK_eab38eb12a3ec6df8376c95477c; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags_closure
    ADD CONSTRAINT "PK_eab38eb12a3ec6df8376c95477c" PRIMARY KEY (id_ancestor, id_descendant);


--
-- Name: tag_asset PK_ef5346fe522b5fb3bc96454747e; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tag_asset
    ADD CONSTRAINT "PK_ef5346fe522b5fb3bc96454747e" PRIMARY KEY ("assetsId", "tagsId");


--
-- Name: partners PK_f1cc8f73d16b367f426261a8736; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT "PK_f1cc8f73d16b367f426261a8736" PRIMARY KEY ("sharedById", "sharedWithId");


--
-- Name: album_users_audit PK_f479a2e575b7ebc9698362c1688; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.album_users_audit
    ADD CONSTRAINT "PK_f479a2e575b7ebc9698362c1688" PRIMARY KEY (id);


--
-- Name: system_metadata PK_fa94f6857470fb5b81ec6084465; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.system_metadata
    ADD CONSTRAINT "PK_fa94f6857470fb5b81ec6084465" PRIMARY KEY (key);


--
-- Name: memories_assets_assets PK_fcaf7112a013d1703c011c6793d; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.memories_assets_assets
    ADD CONSTRAINT "PK_fcaf7112a013d1703c011c6793d" PRIMARY KEY ("memoriesId", "assetsId");


--
-- Name: asset_stack REL_91704e101438fd0653f582426d; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_stack
    ADD CONSTRAINT "REL_91704e101438fd0653f582426d" UNIQUE ("primaryAssetId");


--
-- Name: tags UQ_79d6f16e52bb2c7130375246793; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT "UQ_79d6f16e52bb2c7130375246793" UNIQUE ("userId", value);


--
-- Name: users UQ_97672ac88f789774dd47f7c8be3; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_97672ac88f789774dd47f7c8be3" UNIQUE (email);


--
-- Name: asset_files UQ_assetId_type; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_files
    ADD CONSTRAINT "UQ_assetId_type" UNIQUE ("assetId", type);


--
-- Name: users UQ_b309cf34fa58137c416b32cea3a; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_b309cf34fa58137c416b32cea3a" UNIQUE ("storageLabel");


--
-- Name: move_history UQ_entityId_pathType; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.move_history
    ADD CONSTRAINT "UQ_entityId_pathType" UNIQUE ("entityId", "pathType");


--
-- Name: move_history UQ_newPath; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.move_history
    ADD CONSTRAINT "UQ_newPath" UNIQUE ("newPath");


--
-- Name: shared_links UQ_sharedlink_key; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_links
    ADD CONSTRAINT "UQ_sharedlink_key" UNIQUE (key);


--
-- Name: face_search face_search_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.face_search
    ADD CONSTRAINT face_search_pkey PRIMARY KEY ("faceId");


--
-- Name: geodata_places geodata_places_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.geodata_places
    ADD CONSTRAINT geodata_places_pkey PRIMARY KEY (id) WITH (fillfactor='100');


--
-- Name: kysely_migrations_lock kysely_migrations_lock_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.kysely_migrations_lock
    ADD CONSTRAINT kysely_migrations_lock_pkey PRIMARY KEY (id);


--
-- Name: kysely_migrations kysely_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.kysely_migrations
    ADD CONSTRAINT kysely_migrations_pkey PRIMARY KEY (name);


--
-- Name: naturalearth_countries naturalearth_countries_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.naturalearth_countries
    ADD CONSTRAINT naturalearth_countries_pkey PRIMARY KEY (id) WITH (fillfactor='100');


--
-- Name: smart_search smart_search_pkey; Type: CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.smart_search
    ADD CONSTRAINT smart_search_pkey PRIMARY KEY ("assetId");


--
-- Name: IDX_05895aa505a670300d4816debc; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_05895aa505a670300d4816debc" ON public.albums USING btree ("albumThumbnailAssetId");


--
-- Name: IDX_0f6fc2fb195f24d19b0fb0d57c; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_0f6fc2fb195f24d19b0fb0d57c" ON public.libraries USING btree ("ownerId");


--
-- Name: IDX_15fbcbc67663c6bfc07b354c22; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_15fbcbc67663c6bfc07b354c22" ON public.tags_closure USING btree (id_ancestor);


--
-- Name: IDX_16294b83fa8c0149719a1f631e; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_16294b83fa8c0149719a1f631e" ON public.assets USING btree ("livePhotoVideoId");


--
-- Name: IDX_1af8519996fbfb3684b58df280; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_1af8519996fbfb3684b58df280" ON public.activity USING btree ("albumId");


--
-- Name: IDX_2bbabe31656b6778c6b87b6102; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_2bbabe31656b6778c6b87b6102" ON public.person USING btree ("faceAssetId");


--
-- Name: IDX_2c5ac0d6fb58b238fd2068de67; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_2c5ac0d6fb58b238fd2068de67" ON public.assets USING btree ("ownerId");


--
-- Name: IDX_3571467bcbe021f66e2bdce96e; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_3571467bcbe021f66e2bdce96e" ON public.activity USING btree ("userId");


--
-- Name: IDX_427c350ad49bd3935a50baab73; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_427c350ad49bd3935a50baab73" ON public.albums_shared_users_users USING btree ("albumsId");


--
-- Name: IDX_4bd1303d199f4e72ccdf998c62; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_4bd1303d199f4e72ccdf998c62" ON public.albums_assets_assets USING btree ("assetsId");


--
-- Name: IDX_4d66e76dada1ca180f67a205dc; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_4d66e76dada1ca180f67a205dc" ON public.assets USING btree ("originalFileName");


--
-- Name: IDX_5527cc99f530a547093f9e577b; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_5527cc99f530a547093f9e577b" ON public.person USING btree ("ownerId");


--
-- Name: IDX_575842846f0c28fa5da46c99b1; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_575842846f0c28fa5da46c99b1" ON public.memories USING btree ("ownerId");


--
-- Name: IDX_57de40bc620f456c7311aa3a1e; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_57de40bc620f456c7311aa3a1e" ON public.sessions USING btree ("userId");


--
-- Name: IDX_5b7decce6c8d3db9593d6111a6; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_5b7decce6c8d3db9593d6111a6" ON public.shared_link__asset USING btree ("assetsId");


--
-- Name: IDX_66fe3837414c5a9f1c33ca4934; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_66fe3837414c5a9f1c33ca4934" ON public.shared_links USING btree ("userId");


--
-- Name: IDX_692a909ee0fa9383e7859f9b40; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_692a909ee0fa9383e7859f9b40" ON public.notifications USING btree ("userId");


--
-- Name: IDX_6942ecf52d75d4273de19d2c16; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_6942ecf52d75d4273de19d2c16" ON public.memories_assets_assets USING btree ("assetsId");


--
-- Name: IDX_6c2e267ae764a9413b863a2934; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_6c2e267ae764a9413b863a2934" ON public.api_keys USING btree ("userId");


--
-- Name: IDX_8091ea76b12338cb4428d33d78; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_8091ea76b12338cb4428d33d78" ON public.activity USING btree ("assetId");


--
-- Name: IDX_8d3efe36c0755849395e6ea866; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_8d3efe36c0755849395e6ea866" ON public.assets USING btree (checksum);


--
-- Name: IDX_91704e101438fd0653f582426d; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_91704e101438fd0653f582426d" ON public.asset_stack USING btree ("primaryAssetId");


--
-- Name: IDX_984e5c9ab1f04d34538cd32334; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_984e5c9ab1f04d34538cd32334" ON public.memories_assets_assets USING btree ("memoriesId");


--
-- Name: IDX_9977c3c1de01c3d848039a6b90; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_9977c3c1de01c3d848039a6b90" ON public.assets USING btree ("libraryId");


--
-- Name: IDX_9f9590cc11561f1f48ff034ef9; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_9f9590cc11561f1f48ff034ef9" ON public.tags USING btree ("parentId");


--
-- Name: IDX_activity_like; Type: INDEX; Schema: public; Owner: immich
--

CREATE UNIQUE INDEX "IDX_activity_like" ON public.activity USING btree ("assetId", "userId", "albumId") WHERE ("isLiked" = true);


--
-- Name: IDX_activity_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_activity_update_id" ON public.activity USING btree ("updateId");


--
-- Name: IDX_afbbabbd7daf5b91de4dca84de; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_afbbabbd7daf5b91de4dca84de" ON public.sessions USING btree ("parentId");


--
-- Name: IDX_album_users_audit_album_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_album_users_audit_album_id" ON public.album_users_audit USING btree ("albumId");


--
-- Name: IDX_album_users_audit_deleted_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_album_users_audit_deleted_at" ON public.album_users_audit USING btree ("deletedAt");


--
-- Name: IDX_album_users_audit_user_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_album_users_audit_user_id" ON public.album_users_audit USING btree ("userId");


--
-- Name: IDX_album_users_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_album_users_update_id" ON public.albums_shared_users_users USING btree ("updateId");


--
-- Name: IDX_albums_audit_album_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_albums_audit_album_id" ON public.albums_audit USING btree ("albumId");


--
-- Name: IDX_albums_audit_deleted_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_albums_audit_deleted_at" ON public.albums_audit USING btree ("deletedAt");


--
-- Name: IDX_albums_audit_user_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_albums_audit_user_id" ON public.albums_audit USING btree ("userId");


--
-- Name: IDX_albums_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_albums_update_id" ON public.albums USING btree ("updateId");


--
-- Name: IDX_api_keys_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_api_keys_update_id" ON public.api_keys USING btree ("updateId");


--
-- Name: IDX_asset_exif_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_asset_exif_update_id" ON public.exif USING btree ("updateId");


--
-- Name: IDX_asset_faces_assetId_personId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_asset_faces_assetId_personId" ON public.asset_faces USING btree ("assetId", "personId");


--
-- Name: IDX_asset_files_assetId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_asset_files_assetId" ON public.asset_files USING btree ("assetId");


--
-- Name: IDX_asset_files_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_asset_files_update_id" ON public.asset_files USING btree ("updateId");


--
-- Name: IDX_asset_id_stackId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_asset_id_stackId" ON public.assets USING btree (id, "stackId");


--
-- Name: IDX_assets_audit_asset_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_assets_audit_asset_id" ON public.assets_audit USING btree ("assetId");


--
-- Name: IDX_assets_audit_deleted_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_assets_audit_deleted_at" ON public.assets_audit USING btree ("deletedAt");


--
-- Name: IDX_assets_audit_owner_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_assets_audit_owner_id" ON public.assets_audit USING btree ("ownerId");


--
-- Name: IDX_assets_duplicateId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_assets_duplicateId" ON public.assets USING btree ("duplicateId");


--
-- Name: IDX_assets_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_assets_update_id" ON public.assets USING btree ("updateId");


--
-- Name: IDX_auto_stack_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_auto_stack_id" ON public.exif USING btree ("autoStackId");


--
-- Name: IDX_b1a2a7ed45c29179b5ad51548a; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_b1a2a7ed45c29179b5ad51548a" ON public.tags_closure USING btree (id_descendant);


--
-- Name: IDX_b22c53f35ef20c28c21637c85f; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_b22c53f35ef20c28c21637c85f" ON public.albums USING btree ("ownerId");


--
-- Name: IDX_bf339a24070dac7e71304ec530; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_bf339a24070dac7e71304ec530" ON public.asset_faces USING btree ("personId", "assetId");


--
-- Name: IDX_c05079e542fd74de3b5ecb5c1c; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_c05079e542fd74de3b5ecb5c1c" ON public.asset_stack USING btree ("ownerId");


--
-- Name: IDX_c9fab4aa97ffd1b034f3d6581a; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_c9fab4aa97ffd1b034f3d6581a" ON public.shared_link__asset USING btree ("sharedLinksId");


--
-- Name: IDX_d7e875c6c60e661723dbf372fd; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_d7e875c6c60e661723dbf372fd" ON public.partners USING btree ("sharedWithId");


--
-- Name: IDX_d8ddd9d687816cc490432b3d4b; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_d8ddd9d687816cc490432b3d4b" ON public.session_sync_checkpoints USING btree ("sessionId");


--
-- Name: IDX_e590fa396c6898fcd4a50e4092; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_e590fa396c6898fcd4a50e4092" ON public.albums_assets_assets USING btree ("albumsId");


--
-- Name: IDX_e99f31ea4cdf3a2c35c7287eb4; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_e99f31ea4cdf3a2c35c7287eb4" ON public.tag_asset USING btree ("tagsId");


--
-- Name: IDX_f15d48fa3ea5e4bda05ca8ab20; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_f15d48fa3ea5e4bda05ca8ab20" ON public.assets USING btree ("stackId");


--
-- Name: IDX_f48513bf9bccefd6ff3ad30bd0; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_f48513bf9bccefd6ff3ad30bd0" ON public.albums_shared_users_users USING btree ("usersId");


--
-- Name: IDX_f8e8a9e893cb5c54907f1b798e; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_f8e8a9e893cb5c54907f1b798e" ON public.tag_asset USING btree ("assetsId");


--
-- Name: IDX_libraries_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_libraries_update_id" ON public.libraries USING btree ("updateId");


--
-- Name: IDX_live_photo_cid; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_live_photo_cid" ON public.exif USING btree ("livePhotoCID");


--
-- Name: IDX_memories_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_memories_update_id" ON public.memories USING btree ("updateId");


--
-- Name: IDX_notifications_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_notifications_update_id" ON public.notifications USING btree ("updateId");


--
-- Name: IDX_originalPath_libraryId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_originalPath_libraryId" ON public.assets USING btree ("originalPath", "libraryId");


--
-- Name: IDX_ownerId_createdAt; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_ownerId_createdAt" ON public.audit USING btree ("ownerId", "createdAt");


--
-- Name: IDX_partners_audit_deleted_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_partners_audit_deleted_at" ON public.partners_audit USING btree ("deletedAt");


--
-- Name: IDX_partners_audit_shared_by_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_partners_audit_shared_by_id" ON public.partners_audit USING btree ("sharedById");


--
-- Name: IDX_partners_audit_shared_with_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_partners_audit_shared_with_id" ON public.partners_audit USING btree ("sharedWithId");


--
-- Name: IDX_partners_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_partners_update_id" ON public.partners USING btree ("updateId");


--
-- Name: IDX_person_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_person_update_id" ON public.person USING btree ("updateId");


--
-- Name: IDX_session_sync_checkpoints_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_session_sync_checkpoints_update_id" ON public.session_sync_checkpoints USING btree ("updateId");


--
-- Name: IDX_sessions_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_sessions_update_id" ON public.sessions USING btree ("updateId");


--
-- Name: IDX_sharedlink_albumId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_sharedlink_albumId" ON public.shared_links USING btree ("albumId");


--
-- Name: IDX_sharedlink_key; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_sharedlink_key" ON public.shared_links USING btree (key);


--
-- Name: IDX_tag_asset_assetsId_tagsId; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_tag_asset_assetsId_tagsId" ON public.tag_asset USING btree ("assetsId", "tagsId");


--
-- Name: IDX_tags_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_tags_update_id" ON public.tags USING btree ("updateId");


--
-- Name: IDX_users_audit_deleted_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_users_audit_deleted_at" ON public.users_audit USING btree ("deletedAt");


--
-- Name: IDX_users_update_id; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_users_update_id" ON public.users USING btree ("updateId");


--
-- Name: IDX_users_updated_at_asc_id_asc; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX "IDX_users_updated_at_asc_id_asc" ON public.users USING btree ("updatedAt", id);


--
-- Name: UQ_assets_owner_checksum; Type: INDEX; Schema: public; Owner: immich
--

CREATE UNIQUE INDEX "UQ_assets_owner_checksum" ON public.assets USING btree ("ownerId", checksum) WHERE ("libraryId" IS NULL);


--
-- Name: UQ_assets_owner_library_checksum; Type: INDEX; Schema: public; Owner: immich
--

CREATE UNIQUE INDEX "UQ_assets_owner_library_checksum" ON public.assets USING btree ("ownerId", "libraryId", checksum) WHERE ("libraryId" IS NOT NULL);


--
-- Name: clip_index; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX clip_index ON public.smart_search USING vchordrq (embedding public.vector_cosine_ops) WITH (options='
        residual_quantization = false
        [build.internal]
        lists = [1]
        spherical_centroids = true
        build_threads = 4
        sampling_factor = 1024
        ');


--
-- Name: exif_city; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX exif_city ON public.exif USING btree (city);


--
-- Name: face_index; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX face_index ON public.face_search USING vchordrq (embedding public.vector_cosine_ops) WITH (options='
        residual_quantization = false
        [build.internal]
        lists = [1]
        spherical_centroids = true
        build_threads = 4
        sampling_factor = 1024
        ');


--
-- Name: idx_asset_file_created_at; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_asset_file_created_at ON public.assets USING btree ("fileCreatedAt");


--
-- Name: idx_geodata_gist_earthcoord; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_geodata_gist_earthcoord ON public.geodata_places USING gist (public.ll_to_earth_public(latitude, longitude)) WITH (fillfactor='100');


--
-- Name: idx_geodata_places_admin1_name; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_geodata_places_admin1_name ON public.geodata_places USING gin (public.f_unaccent(("admin1Name")::text) public.gin_trgm_ops);


--
-- Name: idx_geodata_places_admin2_name; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_geodata_places_admin2_name ON public.geodata_places USING gin (public.f_unaccent(("admin2Name")::text) public.gin_trgm_ops);


--
-- Name: idx_geodata_places_alternate_names; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_geodata_places_alternate_names ON public.geodata_places USING gin (public.f_unaccent(("alternateNames")::text) public.gin_trgm_ops);


--
-- Name: idx_geodata_places_name; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_geodata_places_name ON public.geodata_places USING gin (public.f_unaccent((name)::text) public.gin_trgm_ops);


--
-- Name: idx_local_date_time; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_local_date_time ON public.assets USING btree (((("localDateTime" AT TIME ZONE 'UTC'::text))::date));


--
-- Name: idx_local_date_time_month; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_local_date_time_month ON public.assets USING btree ((date_trunc('MONTH'::text, ("localDateTime" AT TIME ZONE 'UTC'::text)) AT TIME ZONE 'UTC'::text));


--
-- Name: idx_originalfilename_trigram; Type: INDEX; Schema: public; Owner: immich
--

CREATE INDEX idx_originalfilename_trigram ON public.assets USING gin (public.f_unaccent(("originalFileName")::text) public.gin_trgm_ops);


--
-- Name: activity activity_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER activity_updated_at BEFORE UPDATE ON public.activity FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: albums_shared_users_users album_user_after_insert; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER album_user_after_insert AFTER INSERT ON public.albums_shared_users_users REFERENCING NEW TABLE AS inserted_rows FOR EACH STATEMENT EXECUTE FUNCTION public.album_user_after_insert();


--
-- Name: albums_shared_users_users album_users_delete_audit; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER album_users_delete_audit AFTER DELETE ON public.albums_shared_users_users REFERENCING OLD TABLE AS old FOR EACH STATEMENT WHEN ((pg_trigger_depth() <= 1)) EXECUTE FUNCTION public.album_users_delete_audit();


--
-- Name: albums_shared_users_users album_users_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER album_users_updated_at BEFORE UPDATE ON public.albums_shared_users_users FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: albums albums_delete_audit; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER albums_delete_audit AFTER DELETE ON public.albums REFERENCING OLD TABLE AS old FOR EACH STATEMENT WHEN ((pg_trigger_depth() = 0)) EXECUTE FUNCTION public.albums_delete_audit();


--
-- Name: albums albums_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER albums_updated_at BEFORE UPDATE ON public.albums FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: api_keys api_keys_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER api_keys_updated_at BEFORE UPDATE ON public.api_keys FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: exif asset_exif_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER asset_exif_updated_at BEFORE UPDATE ON public.exif FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: asset_files asset_files_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER asset_files_updated_at BEFORE UPDATE ON public.asset_files FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: assets assets_delete_audit; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER assets_delete_audit AFTER DELETE ON public.assets REFERENCING OLD TABLE AS old FOR EACH STATEMENT WHEN ((pg_trigger_depth() = 0)) EXECUTE FUNCTION public.assets_delete_audit();


--
-- Name: assets assets_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER assets_updated_at BEFORE UPDATE ON public.assets FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: libraries libraries_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER libraries_updated_at BEFORE UPDATE ON public.libraries FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: memories memories_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER memories_updated_at BEFORE UPDATE ON public.memories FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: notifications notifications_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER notifications_updated_at BEFORE UPDATE ON public.notifications FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: partners partners_delete_audit; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER partners_delete_audit AFTER DELETE ON public.partners REFERENCING OLD TABLE AS old FOR EACH STATEMENT WHEN ((pg_trigger_depth() = 0)) EXECUTE FUNCTION public.partners_delete_audit();


--
-- Name: partners partners_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER partners_updated_at BEFORE UPDATE ON public.partners FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: person person_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER person_updated_at BEFORE UPDATE ON public.person FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: session_sync_checkpoints session_sync_checkpoints_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER session_sync_checkpoints_updated_at BEFORE UPDATE ON public.session_sync_checkpoints FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: sessions sessions_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER sessions_updated_at BEFORE UPDATE ON public.sessions FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: tags tags_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER tags_updated_at BEFORE UPDATE ON public.tags FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: users users_delete_audit; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER users_delete_audit AFTER DELETE ON public.users REFERENCING OLD TABLE AS old FOR EACH STATEMENT WHEN ((pg_trigger_depth() = 0)) EXECUTE FUNCTION public.users_delete_audit();


--
-- Name: users users_updated_at; Type: TRIGGER; Schema: public; Owner: immich
--

CREATE TRIGGER users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.updated_at();


--
-- Name: asset_faces FK_02a43fd0b3c50fb6d7f0cb7282c; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_faces
    ADD CONSTRAINT "FK_02a43fd0b3c50fb6d7f0cb7282c" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: albums FK_05895aa505a670300d4816debce; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums
    ADD CONSTRAINT "FK_05895aa505a670300d4816debce" FOREIGN KEY ("albumThumbnailAssetId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: shared_links FK_0c6ce9058c29f07cdf7014eac66; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_links
    ADD CONSTRAINT "FK_0c6ce9058c29f07cdf7014eac66" FOREIGN KEY ("albumId") REFERENCES public.albums(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: libraries FK_0f6fc2fb195f24d19b0fb0d57c1; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.libraries
    ADD CONSTRAINT "FK_0f6fc2fb195f24d19b0fb0d57c1" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: tags_closure FK_15fbcbc67663c6bfc07b354c22c; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags_closure
    ADD CONSTRAINT "FK_15fbcbc67663c6bfc07b354c22c" FOREIGN KEY (id_ancestor) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: assets FK_16294b83fa8c0149719a1f631ef; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT "FK_16294b83fa8c0149719a1f631ef" FOREIGN KEY ("livePhotoVideoId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: activity FK_1af8519996fbfb3684b58df280b; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT "FK_1af8519996fbfb3684b58df280b" FOREIGN KEY ("albumId") REFERENCES public.albums(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: person FK_2bbabe31656b6778c6b87b61023; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.person
    ADD CONSTRAINT "FK_2bbabe31656b6778c6b87b61023" FOREIGN KEY ("faceAssetId") REFERENCES public.asset_faces(id) ON DELETE SET NULL;


--
-- Name: assets FK_2c5ac0d6fb58b238fd2068de67d; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT "FK_2c5ac0d6fb58b238fd2068de67d" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: activity FK_3571467bcbe021f66e2bdce96ea; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT "FK_3571467bcbe021f66e2bdce96ea" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: asset_job_status FK_420bec36fc02813bddf5c8b73d4; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_job_status
    ADD CONSTRAINT "FK_420bec36fc02813bddf5c8b73d4" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: albums_shared_users_users FK_427c350ad49bd3935a50baab737; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_shared_users_users
    ADD CONSTRAINT "FK_427c350ad49bd3935a50baab737" FOREIGN KEY ("albumsId") REFERENCES public.albums(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: albums_assets_assets FK_4bd1303d199f4e72ccdf998c621; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_assets_assets
    ADD CONSTRAINT "FK_4bd1303d199f4e72ccdf998c621" FOREIGN KEY ("assetsId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: person FK_5527cc99f530a547093f9e577b6; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.person
    ADD CONSTRAINT "FK_5527cc99f530a547093f9e577b6" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: memories FK_575842846f0c28fa5da46c99b19; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.memories
    ADD CONSTRAINT "FK_575842846f0c28fa5da46c99b19" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: sessions FK_57de40bc620f456c7311aa3a1e6; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT "FK_57de40bc620f456c7311aa3a1e6" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: shared_link__asset FK_5b7decce6c8d3db9593d6111a66; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_link__asset
    ADD CONSTRAINT "FK_5b7decce6c8d3db9593d6111a66" FOREIGN KEY ("assetsId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: shared_links FK_66fe3837414c5a9f1c33ca49340; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_links
    ADD CONSTRAINT "FK_66fe3837414c5a9f1c33ca49340" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: notifications FK_692a909ee0fa9383e7859f9b406; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT "FK_692a909ee0fa9383e7859f9b406" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: memories_assets_assets FK_6942ecf52d75d4273de19d2c16f; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.memories_assets_assets
    ADD CONSTRAINT "FK_6942ecf52d75d4273de19d2c16f" FOREIGN KEY ("assetsId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: user_metadata FK_6afb43681a21cf7815932bc38ac; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.user_metadata
    ADD CONSTRAINT "FK_6afb43681a21cf7815932bc38ac" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: api_keys FK_6c2e267ae764a9413b863a29342; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT "FK_6c2e267ae764a9413b863a29342" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: partners FK_7e077a8b70b3530138610ff5e04; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT "FK_7e077a8b70b3530138610ff5e04" FOREIGN KEY ("sharedById") REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: activity FK_8091ea76b12338cb4428d33d782; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.activity
    ADD CONSTRAINT "FK_8091ea76b12338cb4428d33d782" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: asset_stack FK_91704e101438fd0653f582426dc; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_stack
    ADD CONSTRAINT "FK_91704e101438fd0653f582426dc" FOREIGN KEY ("primaryAssetId") REFERENCES public.assets(id);


--
-- Name: tags FK_92e67dc508c705dd66c94615576; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT "FK_92e67dc508c705dd66c94615576" FOREIGN KEY ("userId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: asset_faces FK_95ad7106dd7b484275443f580f9; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_faces
    ADD CONSTRAINT "FK_95ad7106dd7b484275443f580f9" FOREIGN KEY ("personId") REFERENCES public.person(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: memories_assets_assets FK_984e5c9ab1f04d34538cd32334e; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.memories_assets_assets
    ADD CONSTRAINT "FK_984e5c9ab1f04d34538cd32334e" FOREIGN KEY ("memoriesId") REFERENCES public.memories(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: assets FK_9977c3c1de01c3d848039a6b90c; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT "FK_9977c3c1de01c3d848039a6b90c" FOREIGN KEY ("libraryId") REFERENCES public.libraries(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: tags FK_9f9590cc11561f1f48ff034ef99; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT "FK_9f9590cc11561f1f48ff034ef99" FOREIGN KEY ("parentId") REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: sessions FK_afbbabbd7daf5b91de4dca84de8; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT "FK_afbbabbd7daf5b91de4dca84de8" FOREIGN KEY ("parentId") REFERENCES public.sessions(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: tags_closure FK_b1a2a7ed45c29179b5ad51548a1; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tags_closure
    ADD CONSTRAINT "FK_b1a2a7ed45c29179b5ad51548a1" FOREIGN KEY (id_descendant) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: albums FK_b22c53f35ef20c28c21637c85f4; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums
    ADD CONSTRAINT "FK_b22c53f35ef20c28c21637c85f4" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: exif FK_c0117fdbc50b917ef9067740c44; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.exif
    ADD CONSTRAINT "FK_c0117fdbc50b917ef9067740c44" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON DELETE CASCADE;


--
-- Name: asset_stack FK_c05079e542fd74de3b5ecb5c1c8; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_stack
    ADD CONSTRAINT "FK_c05079e542fd74de3b5ecb5c1c8" FOREIGN KEY ("ownerId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: shared_link__asset FK_c9fab4aa97ffd1b034f3d6581ab; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.shared_link__asset
    ADD CONSTRAINT "FK_c9fab4aa97ffd1b034f3d6581ab" FOREIGN KEY ("sharedLinksId") REFERENCES public.shared_links(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: partners FK_d7e875c6c60e661723dbf372fd3; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT "FK_d7e875c6c60e661723dbf372fd3" FOREIGN KEY ("sharedWithId") REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: session_sync_checkpoints FK_d8ddd9d687816cc490432b3d4bc; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.session_sync_checkpoints
    ADD CONSTRAINT "FK_d8ddd9d687816cc490432b3d4bc" FOREIGN KEY ("sessionId") REFERENCES public.sessions(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: asset_files FK_e3e103a5f1d8bc8402999286040; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.asset_files
    ADD CONSTRAINT "FK_e3e103a5f1d8bc8402999286040" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: albums_assets_assets FK_e590fa396c6898fcd4a50e40927; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_assets_assets
    ADD CONSTRAINT "FK_e590fa396c6898fcd4a50e40927" FOREIGN KEY ("albumsId") REFERENCES public.albums(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: tag_asset FK_e99f31ea4cdf3a2c35c7287eb42; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tag_asset
    ADD CONSTRAINT "FK_e99f31ea4cdf3a2c35c7287eb42" FOREIGN KEY ("tagsId") REFERENCES public.tags(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: assets FK_f15d48fa3ea5e4bda05ca8ab207; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.assets
    ADD CONSTRAINT "FK_f15d48fa3ea5e4bda05ca8ab207" FOREIGN KEY ("stackId") REFERENCES public.asset_stack(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: albums_shared_users_users FK_f48513bf9bccefd6ff3ad30bd06; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.albums_shared_users_users
    ADD CONSTRAINT "FK_f48513bf9bccefd6ff3ad30bd06" FOREIGN KEY ("usersId") REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: tag_asset FK_f8e8a9e893cb5c54907f1b798e9; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.tag_asset
    ADD CONSTRAINT "FK_f8e8a9e893cb5c54907f1b798e9" FOREIGN KEY ("assetsId") REFERENCES public.assets(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: face_search face_search_faceId_fkey; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.face_search
    ADD CONSTRAINT "face_search_faceId_fkey" FOREIGN KEY ("faceId") REFERENCES public.asset_faces(id) ON DELETE CASCADE;


--
-- Name: smart_search smart_search_assetId_fkey; Type: FK CONSTRAINT; Schema: public; Owner: immich
--

ALTER TABLE ONLY public.smart_search
    ADD CONSTRAINT "smart_search_assetId_fkey" FOREIGN KEY ("assetId") REFERENCES public.assets(id) ON DELETE CASCADE;

--
-- Name: asset_views; Type: TABLE; Schema: public; Owner: immich
--

CREATE TABLE public.asset_views (
    asset_id uuid NOT NULL,
    user_id uuid NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (asset_id, user_id),
    CONSTRAINT "asset_views_asset_id_fkey" FOREIGN KEY (asset_id) REFERENCES public.assets(id) ON DELETE CASCADE,
    CONSTRAINT "asset_views_user_id_fkey" FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX idx_asset_views_asset_id ON public.asset_views USING btree (asset_id);
CREATE INDEX idx_asset_views_user_id ON public.asset_views USING btree (user_id);
CREATE INDEX idx_asset_views_viewed_at ON public.asset_views USING btree (viewed_at DESC);
