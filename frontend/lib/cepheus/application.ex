defmodule Cepheus.Application do
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    children = [
      CepheusWeb.Telemetry,
      Cepheus.Repo,
      {DNSCluster, query: Application.get_env(:cepheus, :dns_cluster_query) || :ignore},
      {Phoenix.PubSub, name: Cepheus.PubSub},
      CepheusWeb.Endpoint
    ]

    opts = [strategy: :one_for_one, name: Cepheus.Supervisor]
    Supervisor.start_link(children, opts)
  end

  @impl true
  def config_change(changed, _new, removed) do
    CepheusWeb.Endpoint.config_change(changed, removed)
    :ok
  end
end
