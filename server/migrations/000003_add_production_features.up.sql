CREATE TABLE public.announcements (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    title text NOT NULL,
    content text NOT NULL,
    type text DEFAULT 'info'::text NOT NULL,
    priority bigint DEFAULT 0,
    is_active boolean DEFAULT false,
    starts_at timestamp with time zone,
    ends_at timestamp with time zone
);


CREATE TABLE public.async_tasks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL,
    type text NOT NULL,
    status text DEFAULT 'pending'::text,
    input text,
    result text,
    webhook_url text,
    progress bigint DEFAULT 0,
    error text,
    completed_at timestamp with time zone
);


CREATE TABLE public.coupons (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    code character varying(32) NOT NULL,
    name text NOT NULL,
    type text DEFAULT 'percent'::text NOT NULL,
    discount_value numeric NOT NULL,
    min_amount numeric DEFAULT 0,
    max_uses bigint DEFAULT 0,
    use_count bigint DEFAULT 0,
    max_uses_per_user bigint DEFAULT 1,
    is_active boolean DEFAULT true,
    expires_at timestamp with time zone
);


CREATE TABLE public.documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    title text NOT NULL,
    slug text NOT NULL,
    content text,
    category text DEFAULT 'general'::text NOT NULL,
    sort_order bigint DEFAULT 0,
    is_published boolean DEFAULT false
);


CREATE TABLE public.error_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    trajectory_id character varying(255),
    trace_id character varying(255),
    provider character varying(100),
    model character varying(100),
    status_code bigint,
    headers jsonb,
    response_body jsonb,
    created_at timestamp with time zone
);


CREATE TABLE public.integration_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(100),
    enabled boolean,
    config jsonb,
    updated_at timestamp with time zone,
    created_at timestamp with time zone
);


CREATE TABLE public.invite_codes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    code text NOT NULL,
    created_by uuid NOT NULL,
    max_uses bigint DEFAULT 1,
    use_count bigint DEFAULT 0,
    expires_at timestamp with time zone,
    is_active boolean DEFAULT true
);


CREATE TABLE public.mcp_servers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text NOT NULL,
    type text NOT NULL,
    command text,
    args jsonb,
    env jsonb,
    url text,
    is_active boolean DEFAULT true,
    status text DEFAULT 'disconnected'::text,
    last_error text,
    last_checked_at timestamp with time zone
);


CREATE TABLE public.mcp_tools (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    server_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    input_schema jsonb,
    is_active boolean DEFAULT true
);


CREATE TABLE public.orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL,
    plan_id uuid,
    order_no text NOT NULL,
    amount numeric NOT NULL,
    currency text DEFAULT 'USD'::text,
    status text DEFAULT 'pending'::text,
    payment_method text,
    external_id text
);


CREATE TABLE public.password_reset_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone
);


CREATE TABLE public.plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text NOT NULL,
    description text,
    price_month numeric NOT NULL,
    token_limit bigint NOT NULL,
    rate_limit bigint NOT NULL,
    support_level text DEFAULT 'standard'::text,
    is_active boolean DEFAULT true,
    features text
);


CREATE TABLE public.redeem_codes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    code character varying(32) NOT NULL,
    type text DEFAULT 'credit'::text NOT NULL,
    credit_amount numeric DEFAULT 0,
    plan_id uuid,
    plan_days bigint DEFAULT 30,
    used_by_id uuid,
    used_at timestamp with time zone,
    expires_at timestamp with time zone,
    is_active boolean DEFAULT true,
    batch_id text,
    note text
);


CREATE TABLE public.routing_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    model_pattern character varying(255) NOT NULL,
    target_provider_id uuid NOT NULL,
    fallback_provider_id uuid,
    priority integer DEFAULT 0 NOT NULL,
    is_enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


CREATE TABLE public.subscriptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    status text DEFAULT 'active'::text,
    current_period_start timestamp with time zone,
    current_period_end timestamp with time zone,
    cancel_at_period_end boolean DEFAULT false
);


CREATE TABLE public.system_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    key text NOT NULL,
    value text,
    description text,
    category text,
    is_secret boolean DEFAULT false
);


CREATE TABLE public.transactions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL,
    type text NOT NULL,
    amount numeric NOT NULL,
    currency text DEFAULT 'USD'::text,
    balance numeric,
    description text,
    reference_id text
);


ALTER TABLE ONLY public.announcements
    ADD CONSTRAINT announcements_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.async_tasks
    ADD CONSTRAINT async_tasks_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.coupons
    ADD CONSTRAINT coupons_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.error_logs
    ADD CONSTRAINT error_logs_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.integration_configs
    ADD CONSTRAINT integration_configs_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.invite_codes
    ADD CONSTRAINT invite_codes_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.mcp_servers
    ADD CONSTRAINT mcp_servers_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.mcp_tools
    ADD CONSTRAINT mcp_tools_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.plans
    ADD CONSTRAINT plans_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.redeem_codes
    ADD CONSTRAINT redeem_codes_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.routing_rules
    ADD CONSTRAINT routing_rules_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.system_configs
    ADD CONSTRAINT system_configs_pkey PRIMARY KEY (id);


ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


CREATE INDEX idx_announcements_deleted_at ON public.announcements USING btree (deleted_at);


CREATE INDEX idx_async_tasks_deleted_at ON public.async_tasks USING btree (deleted_at);


CREATE INDEX idx_async_tasks_status ON public.async_tasks USING btree (status);


CREATE INDEX idx_async_tasks_type ON public.async_tasks USING btree (type);


CREATE INDEX idx_async_tasks_user_id ON public.async_tasks USING btree (user_id);


CREATE UNIQUE INDEX idx_coupons_code ON public.coupons USING btree (code);


CREATE INDEX idx_coupons_deleted_at ON public.coupons USING btree (deleted_at);


CREATE INDEX idx_documents_category ON public.documents USING btree (category);


CREATE INDEX idx_documents_deleted_at ON public.documents USING btree (deleted_at);


CREATE UNIQUE INDEX idx_documents_slug ON public.documents USING btree (slug);


CREATE INDEX idx_error_logs_created_at ON public.error_logs USING btree (created_at);


CREATE INDEX idx_error_logs_model ON public.error_logs USING btree (model);


CREATE INDEX idx_error_logs_provider ON public.error_logs USING btree (provider);


CREATE INDEX idx_error_logs_trace_id ON public.error_logs USING btree (trace_id);


CREATE INDEX idx_error_logs_trajectory_id ON public.error_logs USING btree (trajectory_id);


CREATE UNIQUE INDEX idx_integration_configs_name ON public.integration_configs USING btree (name);


CREATE UNIQUE INDEX idx_invite_codes_code ON public.invite_codes USING btree (code);


CREATE INDEX idx_invite_codes_deleted_at ON public.invite_codes USING btree (deleted_at);


CREATE INDEX idx_mcp_servers_deleted_at ON public.mcp_servers USING btree (deleted_at);


CREATE UNIQUE INDEX idx_mcp_servers_name ON public.mcp_servers USING btree (name);


CREATE INDEX idx_mcp_tools_deleted_at ON public.mcp_tools USING btree (deleted_at);


CREATE INDEX idx_mcp_tools_server_id ON public.mcp_tools USING btree (server_id);


CREATE INDEX idx_orders_deleted_at ON public.orders USING btree (deleted_at);


CREATE INDEX idx_orders_external_id ON public.orders USING btree (external_id);


CREATE UNIQUE INDEX idx_orders_order_no ON public.orders USING btree (order_no);


CREATE INDEX idx_orders_plan_id ON public.orders USING btree (plan_id);


CREATE INDEX idx_orders_user_id ON public.orders USING btree (user_id);


CREATE INDEX idx_password_reset_tokens_created_at ON public.password_reset_tokens USING btree (created_at);


CREATE UNIQUE INDEX idx_password_reset_tokens_token_hash ON public.password_reset_tokens USING btree (token_hash);


CREATE INDEX idx_password_reset_tokens_user_id ON public.password_reset_tokens USING btree (user_id);


CREATE INDEX idx_plans_deleted_at ON public.plans USING btree (deleted_at);


CREATE UNIQUE INDEX idx_plans_name ON public.plans USING btree (name);


CREATE INDEX idx_redeem_codes_batch_id ON public.redeem_codes USING btree (batch_id);


CREATE UNIQUE INDEX idx_redeem_codes_code ON public.redeem_codes USING btree (code);


CREATE INDEX idx_redeem_codes_deleted_at ON public.redeem_codes USING btree (deleted_at);


CREATE INDEX idx_redeem_codes_plan_id ON public.redeem_codes USING btree (plan_id);


CREATE INDEX idx_redeem_codes_used_by_id ON public.redeem_codes USING btree (used_by_id);


CREATE INDEX idx_routing_rules_deleted_at ON public.routing_rules USING btree (deleted_at);


CREATE INDEX idx_subscriptions_deleted_at ON public.subscriptions USING btree (deleted_at);


CREATE UNIQUE INDEX idx_subscriptions_user_id ON public.subscriptions USING btree (user_id);


CREATE INDEX idx_system_configs_category ON public.system_configs USING btree (category);


CREATE INDEX idx_system_configs_deleted_at ON public.system_configs USING btree (deleted_at);


CREATE UNIQUE INDEX idx_system_configs_key ON public.system_configs USING btree (key);


CREATE INDEX idx_transactions_deleted_at ON public.transactions USING btree (deleted_at);


CREATE INDEX idx_transactions_reference_id ON public.transactions USING btree (reference_id);


CREATE INDEX idx_transactions_type ON public.transactions USING btree (type);


CREATE INDEX idx_transactions_user_id ON public.transactions USING btree (user_id);


ALTER TABLE ONLY public.mcp_tools
    ADD CONSTRAINT fk_mcp_servers_tools FOREIGN KEY (server_id) REFERENCES public.mcp_servers(id);


ALTER TABLE ONLY public.redeem_codes
    ADD CONSTRAINT fk_redeem_codes_plan FOREIGN KEY (plan_id) REFERENCES public.plans(id);


ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT fk_subscriptions_plan FOREIGN KEY (plan_id) REFERENCES public.plans(id);
