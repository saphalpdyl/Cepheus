import Config

config :cepheus,
  ecto_repos: [Cepheus.Repo],
  generators: [timestamp_type: :utc_datetime]

config :cepheus, CepheusWeb.Endpoint,
  url: [host: "localhost"],
  adapter: Bandit.PhoenixAdapter,
  render_errors: [
    formats: [html: CepheusWeb.ErrorHTML, json: CepheusWeb.ErrorJSON],
    layout: false
  ],
  pubsub_server: Cepheus.PubSub,
  live_view: [signing_salt: "t347Ldnp"]

config :esbuild,
  version: "0.25.4",
  cepheus: [
    args:
      ~w(js/app.js --bundle --target=es2022 --outdir=../priv/static/assets/js --external:/fonts/* --external:/images/* --alias:@=.),
    cd: Path.expand("../assets", __DIR__),
    env: %{"NODE_PATH" => [Path.expand("../deps", __DIR__), Mix.Project.build_path()]}
  ]

config :tailwind,
  version: "4.1.12",
  cepheus: [
    args: ~w(
      --input=assets/css/app.css
      --output=priv/static/assets/css/app.css
    ),
    cd: Path.expand("..", __DIR__)
  ]

config :logger, :default_formatter,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

config :phoenix, :json_library, Jason

import_config "#{config_env()}.exs"
