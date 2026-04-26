import Config

config :cepheus, Cepheus.Repo,
  username: "postgres",
  password: "postgres",
  hostname: "localhost",
  database: "cepheus_test#{System.get_env("MIX_TEST_PARTITION")}",
  pool: Ecto.Adapters.SQL.Sandbox,
  pool_size: System.schedulers_online() * 2

config :cepheus, CepheusWeb.Endpoint,
  http: [ip: {127, 0, 0, 1}, port: 4002],
  secret_key_base: "ssEGApDXwy9CBbwjKxNptTmjC3aQpYNkDwFLcCnhB/KqPq2LumhzN1bi96/ezaSQ",
  server: false

config :logger, level: :warning

config :phoenix, :plug_init_mode, :runtime

config :phoenix_live_view,
  enable_expensive_runtime_checks: true

config :phoenix,
  sort_verified_routes_query_params: true
