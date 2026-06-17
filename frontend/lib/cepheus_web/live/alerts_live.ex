defmodule CepheusWeb.AlertsLive do
  @moduledoc """
  Detection-event inbox — fleet-wide Tier-1 events with status/severity/text
  filters and a triage detail panel.
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
      |> assign(:page_title, "Cepheus · Alerts")
      |> assign(:status_filter, "all")
      |> assign(:sev_filter, "all")
      |> assign(:query, "")
      |> assign(:selected_event_id, nil)
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

  def handle_event("filter_status", %{"value" => value}, socket),
    do: {:noreply, socket |> assign(:status_filter, value) |> load()}

  def handle_event("filter_sev", %{"value" => value}, socket),
    do: {:noreply, socket |> assign(:sev_filter, value) |> load()}

  def handle_event("search", %{"q" => q}, socket),
    do: {:noreply, socket |> assign(:query, q) |> load()}

  def handle_event("select_event", %{"id" => id}, socket),
    do: {:noreply, socket |> assign(:selected_event_id, id) |> load()}

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load(socket) do
    %{window: window, status_filter: status, sev_filter: sev, query: q} = socket.assigns

    events = Dashboard.all_events(window)
    filtered = apply_filters(events, status, sev, q)
    selected_id = pick_selected(socket.assigns.selected_event_id, filtered, events)

    socket
    |> assign(:events, events)
    |> assign(:filtered, filtered)
    |> assign(:open_count, Enum.count(events, &(&1.status == "open")))
    |> assign(:selected_event_id, selected_id)
    |> assign(:detail, selected_id && Dashboard.event_detail(selected_id))
    |> assign(:open_alerts_count, Dashboard.total_open_events())
  end

  # Keep the current selection if it's still visible; otherwise fall back to the
  # first filtered row, then any event at all.
  defp pick_selected(current, filtered, all) do
    ids = MapSet.new(Enum.map(filtered, & &1.id))

    cond do
      current && MapSet.member?(ids, current) -> current
      filtered != [] -> hd(filtered).id
      all != [] -> hd(all).id
      true -> nil
    end
  end

  defp apply_filters(events, status, sev, q) do
    qn = q |> to_string() |> String.downcase() |> String.trim()

    Enum.filter(events, fn e ->
      status_ok?(e, status) and sev_ok?(e, sev) and query_ok?(e, qn)
    end)
  end

  defp status_ok?(_e, "all"), do: true
  defp status_ok?(e, "open"), do: e.status == "open"
  defp status_ok?(e, "resolved"), do: e.status == "closed"
  defp status_ok?(_e, _), do: true

  defp sev_ok?(_e, "all"), do: true
  defp sev_ok?(e, "crit"), do: sev_val(e) >= 4.0
  defp sev_ok?(e, "warn"), do: sev_val(e) >= 3.0 and sev_val(e) < 4.0
  defp sev_ok?(e, "watch"), do: sev_val(e) < 3.0
  defp sev_ok?(_e, _), do: true

  defp sev_val(%{peak_severity: s}) when is_number(s), do: s
  defp sev_val(_), do: 0.0

  defp query_ok?(_e, ""), do: true

  defp query_ok?(e, q) do
    String.contains?(String.downcase("#{e.serial_id} #{e.target} #{e.metric}"), q)
  end
end
