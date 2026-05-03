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

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load_index(socket) do
    devices = Dashboard.list_devices()
    window = socket.assigns.window

    summaries =
      Map.new(devices, fn d ->
        {d.serial_id, Dashboard.summary_by_target(d.serial_id, window)}
      end)

    socket
    |> assign(:devices, devices)
    |> assign(:summary_by_serial, summaries)
    |> assign(:last_loaded_at, DateTime.utc_now())
  end

  defp load_show(socket) do
    %{device: device, window: window} = socket.assigns
    tasks = Dashboard.list_tasks_for_config(device.agent_config_id)
    summary = Dashboard.summary_by_target(device.serial_id, window)
    series = Dashboard.measurement_timeseries(device.serial_id, window)

    socket
    |> assign(:tasks, tasks)
    |> assign(:summary, summary)
    |> assign(:kpis, compute_kpis(summary))
    |> assign(:last_loaded_at, DateTime.utc_now())
    |> push_event("rtt-chart:update", %{series: series})
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
end
