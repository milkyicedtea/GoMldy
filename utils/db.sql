create user melody;

-- Don't use this when deploying!
-- Generating a password with a tool is recommended,
-- But you can also just your own i guess ¯\_(ツ)_/¯
alter user melody with encrypted password 'mldypassword';

create schema if not exists mldy;
grant all on schema mldy to melody; -- Can tweak this however you want
grant all on all tables in schema mldy to melody; -- This might be optimal for dedicated user
grant all on all sequences in schema mldy to melody;
set search_path to mldy;

create table download_rate_limits (
    hashed_ip varchar(96) primary key not null,
    download_count integer not null default 1,
    last_reset timestamp with time zone not null default current_timestamp
);

create index idx_dwnld_rate_limits_last_reset on download_rate_limits(last_reset);

alter table mldy.download_rate_limits owner to melody;