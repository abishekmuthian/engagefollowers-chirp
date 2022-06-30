CREATE TABLE users
(
    id                   SERIAL NOT NULL,
    created_at           timestamp,
    updated_at           timestamp,
    status               integer,
    role                 integer,
    email                text UNIQUE,
    name                 text,
    summary              text,
    notification         boolean default true,
    password_hash        text,
    password_reset_at    timestamp,
    password_reset_token text,
    approved_email       text,
    personal_email       text,
    plan                 text,
    subscription         boolean,
    trial_end            timestamp,
    keywords             text[],
    twitter_connected    boolean,
    twitter_id           text,
    twitter_username     text,
    twitter_access_token text,
    twitter_refresh_token text,
    twitter_token_expiry_time timestamp,
    twitter_list_id       text,
    twitter_followers     text[],
    twitter_list_creation_time timestamp,
    auto_like             boolean
);

CREATE TABLE subscriptions
(
    id                     SERIAL NOT NULL,
    created_at             timestamp,
    updated_at             timestamp,
    txn_id                 text,
    txn_type               text,
    transaction_subject    text,
    business               text,
    custom                 text,
    invoice                text,
    receipt_ID             text,
    first_name             text,
    handling_amount        real,
    item_number            text,
    item_name              text,
    last_name              text,
    mc_currency            text,
    mc_fee                 real,
    mc_gross               real,
    payer_email            text,
    payer_id               text,
    payer_status           text,
    payment_date           timestamp,
    payment_fee            real,
    payment_gross          real,
    payment_status         text,
    payment_type           text,
    protection_eligibility text,
    quantity               integer,
    receiver_id            text,
    receiver_email         text,
    residence_country      text,
    shipping               real,
    tax                    real,
    address_country        text,
    test_ipn               integer,
    address_status         text,
    address_street         text,
    notify_version         real,
    address_city           text,
    verify_sign            text,
    address_state          text,
    charset                text,
    address_name           text,
    address_country_code   text,
    address_zip            integer,
    subscr_id              text,
    user_id                integer,
    test_pdt               integer
);

ALTER TABLE users OWNER TO postgres;
grant
all
on schema public to public;
GRANT INSERT,
UPDATE,
SELECT
ON ALL TABLES IN SCHEMA public TO engagefollowers;
GRANT
USAGE,
SELECT
ON ALL SEQUENCES IN SCHEMA public TO engagefollowers;