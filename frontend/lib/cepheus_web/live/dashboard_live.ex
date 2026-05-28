defmodule CepheusWeb.DashboardLive do
  use CepheusWeb, :live_view

  alias Cepheus.Dashboard

  @refresh_ms 2_000
  @default_window "5 minutes"

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: schedule_tick()

    socket =
      socket
      |> assign(:window, @default_window)
      |> assign(:windows, Dashboard.windows())
      |> assign(:page_title, "Cepheus")
      |> assign(:open_event_counts, %{})
      |> assign(:events, [])
      |> assign(:findings, [])
      |> assign(:stamp_targets, [])
      |> assign(:ping_targets, [])
      |> assign(:selected_stamp_targets, nil)
      |> assign(:selected_ping_targets, nil)
      |> assign(:ping_summary, [])
      |> assign(:ping_kpis, empty_ping_kpis())

    {:ok, socket}
  end

  @impl true
  def handle_params(params, _uri, socket) do
    {:noreply, apply_action(socket, socket.assigns.live_action, params)}
  end

  defp apply_action(socket, :index, _params) do
    socket
    |> assign(:device, nil)
    |> load_index()
  end

  defp apply_action(socket, :show, %{"serial_id" => serial_id}) do
    case Dashboard.get_device(serial_id) do
      nil ->
        socket
        |> put_flash(:error, "Device #{serial_id} not found")
        |> push_navigate(to: ~p"/")

      device ->
        socket
        |> assign(:device, device)
        |> assign(:page_title, "Cepheus · #{device.serial_id}")
        |> load_show()
    end
  end

  @impl true
  def handle_info(:tick, socket) do
    schedule_tick()

    socket =
      case socket.assigns.live_action do
        :index -> load_index(socket)
        :show -> load_show(socket)
      end

    {:noreply, socket}
  end

  @impl true
  def handle_event("set_window", %{"window" => window}, socket) do
    if Dashboard.valid_window?(window) do
      socket = assign(socket, :window, window)

      socket =
        case socket.assigns.live_action do
          :index -> load_index(socket)
          :show -> load_show(socket)
        end

      {:noreply, socket}
    else
      {:noreply, socket}
    end
  end

  def handle_event("set_stamp_targets", params, socket) do
    selected = params |> Map.get("targets", []) |> List.wrap()

    socket =
      socket
      |> assign(:selected_stamp_targets, selected)
      |> load_show()

    {:noreply, socket}
  end

  def handle_event("set_ping_targets", params, socket) do
    selected = params |> Map.get("targets", []) |> List.wrap()

    socket =
      socket
      |> assign(:selected_ping_targets, selected)
      |> load_show()

    {:noreply, socket}
  end

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load_index(socket) do
    devices = Dashboard.list_devices()
    window = socket.assigns.window

    summaries =
      Map.new(devices, fn d ->
        {d.serial_id, Dashboard.summary_by_target(d.serial_id, window)}
      end)

    open_event_counts = Dashboard.open_event_counts_by_device()

    socket
    |> assign(:devices, devices)
    |> assign(:summary_by_serial, summaries)
    |> assign(:open_event_counts, open_event_counts)
    |> assign(:last_loaded_at, DateTime.utc_now())
  end

  defp load_show(socket) do
    %{
      device: device,
      window: window,
      selected_stamp_targets: sel_stamp,
      selected_ping_targets: sel_ping
    } = socket.assigns

    tasks = Dashboard.list_tasks_for_config(device.agent_config_id)
    stamp_targets = Dashboard.list_stamp_targets(device.serial_id)
    ping_targets = Dashboard.list_ping_targets(device.serial_id)

    summary = Dashboard.summary_by_target(device.serial_id, window, sel_stamp)
    series = Dashboard.measurement_timeseries(device.serial_id, window, sel_stamp)
    ping_summary = Dashboard.ping_summary_by_target(device.serial_id, window, sel_ping)
    ping_series = Dashboard.ping_timeseries(device.serial_id, window, sel_ping)
    events = Dashboard.events_for_device(device.serial_id, window)
    findings = Dashboard.recent_findings_for_device(device.serial_id, window)

    socket
    |> assign(:tasks, tasks)
    |> assign(:stamp_targets, stamp_targets)
    |> assign(:ping_targets, ping_targets)
    |> assign(:summary, summary)
    |> assign(:ping_summary, ping_summary)
    |> assign(:kpis, compute_kpis(summary))
    |> assign(:ping_kpis, compute_ping_kpis(ping_summary))
    |> assign(:events, events)
    |> assign(:findings, findings)
    |> assign(:last_loaded_at, DateTime.utc_now())
    |> push_event("rtt-chart:update", %{series: series})
    |> push_event("ping-chart:update", %{series: ping_series})
  end

  defp compute_kpis(summary) do
    targets = length(summary)
    measurements = Enum.reduce(summary, 0, &(&1.measurements + &2))

    avg_p95_ns =
      case Enum.filter(summary, & &1.avg_rtt_p95_ns) do
        [] -> nil
        rows -> Enum.sum(Enum.map(rows, & &1.avg_rtt_p95_ns)) |> div(length(rows))
      end

    worst_loss =
      case Enum.filter(summary, & &1.loss) do
        [] -> nil
        rows -> rows |> Enum.map(& &1.loss) |> Enum.max()
      end

    %{
      targets: targets,
      measurements: measurements,
      avg_p95_ns: avg_p95_ns,
      worst_loss: worst_loss
    }
  end

  defp compute_ping_kpis(summary) do
    targets = length(summary)
    measurements = Enum.reduce(summary, 0, &(&1.measurements + &2))

    avg_rtt_ns =
      case Enum.filter(summary, & &1.avg_rtt_ns) do
        [] -> nil
        rows -> Enum.sum(Enum.map(rows, & &1.avg_rtt_ns)) |> div(length(rows))
      end

    worst_loss =
      case Enum.filter(summary, & &1.loss) do
        [] -> nil
        rows -> rows |> Enum.map(& &1.loss) |> Enum.max()
      end

    %{
      targets: targets,
      measurements: measurements,
      avg_rtt_ns: avg_rtt_ns,
      worst_loss: worst_loss
    }
  end

  defp empty_ping_kpis,
    do: %{targets: 0, measurements: 0, avg_rtt_ns: nil, worst_loss: nil}
end
