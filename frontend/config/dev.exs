import Config

config :cepheus, Cepheus.Repo,
  username: "postgres",
  password: "admin",
  hostname: System.get_env("DB_HOST", "localhost"),
  port: String.to_integer(System.get_env("DB_PORT", "5435")),
  database: "postgres",
  stacktrace: true,
  show_sensitive_data_on_connection_error: true,
  pool_size: 5

config :cepheus, CepheusWeb.Endpoint,
  http: [ip: {0, 0, 0, 0}],
  check_origin: false,
  code_reloader: true,
  debug_errors: true,
  secret_key_base: "KOEVVhWAF/UJLOltvQLwpv6hamFKT6RB2QkwJo/3zUbyahM6ucd0L9N10/M842Ul",
  watchers: [
    esbuild: {Esbuild, :install_and_run, [:cepheus, ~w(--sourcemap=inline --watch)]},
    tailwind: {Tailwind, :install_and_run, [:cepheus, ~w(--watch)]}
  ]

config :cepheus, CepheusWeb.Endpoint,
  live_reload: [
    web_console_logger: true,
    patterns: [
      ~r"priv/static/(?!uploads/).*\.(js|css|png|jpeg|jpg|gif|svg)$",
      ~r"lib/cepheus_web/router\.ex$",
      ~r"lib/cepheus_web/(controllers|live|components)/.*\.(ex|heex)$"
    ]
  ]

config :cepheus, dev_routes: true

config :logger, :default_formatter, format: "[$level] $message\n"

config :phoenix, :stacktrace_depth, 20

config :phoenix, :plug_init_mode, :runtime

config :phoenix_live_view,
  debug_heex_annotations: true,
  debug_attributes: true,
  enable_expensive_runtime_checks: true
