-- Enable the pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create semantic_caches table
CREATE TABLE public.semantic_caches (
    id character varying(36) NOT NULL,
    hash character varying(64) NOT NULL,
    embedding vector(1536) NOT NULL,
    response jsonb NOT NULL,
    provider character varying(64) NOT NULL,
    model character varying(64) NOT NULL,
    metadata jsonb,
    hit_count integer DEFAULT 0,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);

ALTER TABLE ONLY public.semantic_caches
    ADD CONSTRAINT idx_semantic_caches_pkey PRIMARY KEY (id);

CREATE UNIQUE INDEX idx_semantic_caches_hash ON public.semantic_caches USING btree (hash);
CREATE INDEX idx_semantic_caches_embedding ON public.semantic_caches USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_semantic_caches_deleted_at ON public.semantic_caches USING btree (deleted_at);
