defmodule CepheusWeb.FleetLive do
  @moduledoc """
  Network overview — fleet-scale Tier-1 detection: per-family signal summary,
  network heartbeat, open events, probe-path strips and the agents table.
  """
  use CepheusWeb, :live_view

  alias Cepheus.Dashboard

  @refresh_ms 2_000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: schedule_tick()

    socket =
      socket
      |> assign(:window, Dashboard.default_window())
      |> assign(:windows, Dashboard.windows())
      |> assign(:page_title, "Cepheus · Fleet overview")
      |> load()

    {:ok, socket}
  end

  @impl true
  def handle_info(:tick, socket) do
    schedule_tick()
    {:noreply, load(socket)}
  end

  @impl true
  def handle_event("set_window", %{"window" => window}, socket) do
    if Dashboard.valid_window?(window),
      do: {:noreply, socket |> assign(:window, window) |> load()},
      else: {:noreply, socket}
  end

  def handle_event("refresh", _params, socket), do: {:noreply, load(socket)}

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load(socket) do
    window = socket.assigns.window
    agents = Dashboard.fleet_agents(window)
    heartbeat = Dashboard.heartbeat_from_agents(agents)

    open_events = window |> Dashboard.all_events() |> Enum.filter(&(&1.status == "open"))

    socket
    |> assign(:agents, agents)
    |> assign(:heartbeat, heartbeat)
    |> assign(:families, Dashboard.family_stats(window))
    |> assign(:open_events, open_events)
    |> assign(:open_events_shown, Enum.take(open_events, 5))
    |> assign(:probe_paths, Dashboard.probe_paths(window))
    |> assign(:open_alerts_count, Dashboard.total_open_events())
    |> assign(:total_targets, agents |> Enum.map(& &1.targets) |> Enum.sum())
  end
end
