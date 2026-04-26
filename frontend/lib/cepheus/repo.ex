defmodule Cepheus.Repo do
  use Ecto.Repo,
    otp_app: :cepheus,
    adapter: Ecto.Adapters.Postgres
end
