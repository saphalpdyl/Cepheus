defmodule CepheusWeb.DashboardLive do
  use CepheusWeb, :live_view

  alias CepheusWeb.DashboardLive
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
      |> load_all()

    {:ok, socket}
  end

  @impl true
  def handle_info(:tick, socket) do
    schedule_tick()
    {:noreply, load_all(socket)}
  end

  @impl true
  def handle_event("set_window", %{"window" => window}, socket) do
    if Dashboard.valid_window?(window) do
      {:noreply, socket |> assign(:window, window) |> load_all()}
    else
      {:noreply, socket}
    end
  end

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load_all(socket) do
    devices = Dashboard.list_devices()
    window = socket.assigns.window

    tasks_by_config_id =
      devices
      |> Enum.map(& &1.agent_config_id)
      |> Enum.uniq()
      |> Map.new(fn cfg_id -> {cfg_id, Dashboard.list_tasks_for_config(cfg_id)} end)

    rtt_by_serial =
      Map.new(devices, fn d ->
        {d.serial_id, Dashboard.avg_rtt_by_target(d.serial_id, window)}
      end)

    socket
    |> assign(:devices, devices)
    |> assign(:tasks_by_config_id, tasks_by_config_id)
    |> assign(:rtt_by_serial, rtt_by_serial)
    |> assign(:last_loaded_at, DateTime.utc_now())

  end

  defp short_uuid(nil), do: ""
  defp short_uuid(uuid) when is_binary(uuid), do: String.slice(uuid, 0, 8) <> "…"

  defp fmt_dt(%DateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S UTC")
  defp fmt_dt(%NaiveDateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")
  defp fmt_dt(nil), do: "—"
  defp fmt_dt(other), do: to_string(other)

  defp fmt_loss(nil), do: "—"
  defp fmt_loss(f) when is_float(f), do: :io_lib.format("~.2f", [f * 100]) |> IO.iodata_to_binary() |> Kernel.<>("%")
  defp fmt_loss(other), do: to_string(other)

  defp fmt_rtt(nil), do: "—"
  defp fmt_rtt(n) when is_integer(n), do: Integer.to_string(n) <> " µs"
  defp fmt_rtt(other), do: to_string(other)
end
