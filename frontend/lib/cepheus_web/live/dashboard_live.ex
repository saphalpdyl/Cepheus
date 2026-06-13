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
      |> assign(:expanded_findings, MapSet.new())
      |> assign(:stamp_targets, [])
      |> assign(:ping_targets, [])
      |> assign(:selected_stamp_targets, nil)
      |> assign(:selected_ping_targets, nil)
      |> assign(:ping_summary, [])
      |> assign(:ping_kpis, empty_ping_kpis())
      |> assign(:task_rows, [])

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

  def handle_event("toggle_finding_group", %{"key" => key}, socket) do
    expanded = socket.assigns.expanded_findings

    expanded =
      if MapSet.member?(expanded, key),
        do: MapSet.delete(expanded, key),
        else: MapSet.put(expanded, key)

    {:noreply, assign(socket, :expanded_findings, expanded)}
  end

  # ----- Task editor -----

  def handle_event("add_task_row", params, socket) do
    types = Enum.map(Dashboard.task_types(), &elem(&1, 1))
    type = Map.get(params, "type", "trace")
    type = if type in types, do: type, else: "trace"

    row = %{
      ref: "row-" <> Integer.to_string(System.unique_integer([:positive])),
      type: type,
      target: Map.get(params, "target", ""),
      port: Dashboard.default_port(type),
      interval: ""
    }

    {:noreply, assign(socket, :task_rows, socket.assigns.task_rows ++ [row])}
  end

  def handle_event("remove_task_row", %{"ref" => ref}, socket) do
    rows = Enum.reject(socket.assigns.task_rows, &(&1.ref == ref))
    {:noreply, assign(socket, :task_rows, rows)}
  end

  def handle_event("task_form_change", %{"tasks" => task_params}, socket) do
    rows =
      Enum.map(socket.assigns.task_rows, fn row ->
        case Map.get(task_params, row.ref) do
          nil ->
            row

          attrs ->
            %{
              row
              | type: Map.get(attrs, "type", row.type),
                target: Map.get(attrs, "target", row.target),
                port: Map.get(attrs, "port", row.port),
                interval: Map.get(attrs, "interval", row.interval)
            }
        end
      end)

    {:noreply, assign(socket, :task_rows, rows)}
  end

  def handle_event("task_form_change", _params, socket), do: {:noreply, socket}

  def handle_event("save_tasks", _params, socket) do
    device = socket.assigns.device

    with {:ok, tasks} <- build_tasks(socket.assigns.task_rows, device.agent_config_id),
         {:ok, generation} <- Dashboard.add_tasks(device.agent_config_id, tasks) do
      socket =
        socket
        |> assign(:task_rows, [])
        |> assign(:device, Dashboard.get_device(device.serial_id))
        |> put_flash(:info, "Added #{length(tasks)} task(s) · agent now on generation #{generation}")
        |> load_show()

      {:noreply, socket}
    else
      {:error, message} -> {:noreply, put_flash(socket, :error, message)}
    end
  end

  defp build_tasks([], _config_id), do: {:error, "Add at least one task before saving."}

  defp build_tasks(rows, config_id) do
    used =
      config_id
      |> Dashboard.list_tasks_for_config()
      |> Enum.map(& &1.task_id)
      |> MapSet.new()

    result =
      Enum.reduce_while(rows, {[], used}, fn row, {acc, used} ->
        case build_task(row, used) do
          {:ok, task} -> {:cont, {[task | acc], MapSet.put(used, task.task_id)}}
          {:error, message} -> {:halt, {:error, message}}
        end
      end)

    case result do
      {:error, message} -> {:error, message}
      {tasks, _used} -> {:ok, Enum.reverse(tasks)}
    end
  end

  defp build_task(row, used_ids) do
    type = row.type
    target = String.trim(row.target || "")

    with :ok <- validate_type(type),
         :ok <- validate_target(type, target),
         {:ok, port} <- validate_port(type, row.port),
         {:ok, interval} <- validate_interval(type, row.interval) do
      {params, schedule} = task_spec(type, target, port, interval)
      task_id = unique_task_id(type, target, port, used_ids)
      {:ok, Map.merge(%{task_id: task_id, type: type, params: params}, schedule)}
    end
  end

  defp validate_type(type) do
    if type in Enum.map(Dashboard.task_types(), &elem(&1, 1)),
      do: :ok,
      else: {:error, "Unknown task type #{inspect(type)}."}
  end

  defp validate_target(type, target) do
    cond do
      not Dashboard.task_has_target?(type) -> :ok
      target == "" -> {:error, "#{Dashboard.task_type_label(type)}: a target is required."}
      Regex.match?(~r/^[A-Za-z0-9._:\-]+$/, target) -> :ok
      true -> {:error, "#{Dashboard.task_type_label(type)}: #{target} is not a valid target."}
    end
  end

  defp validate_port(type, raw) do
    if Dashboard.task_has_port?(type) do
      value = raw |> to_string() |> String.trim()
      value = if value == "", do: "862", else: value

      case Integer.parse(value) do
        {n, ""} when n >= 1 and n <= 65535 -> {:ok, n}
        _ -> {:error, "#{Dashboard.task_type_label(type)}: port must be between 1 and 65535."}
      end
    else
      {:ok, nil}
    end
  end

  defp validate_interval(type, raw) do
    if Dashboard.task_schedulable?(type) do
      value = raw |> to_string() |> String.trim()

      if value == "" do
        {:ok, Dashboard.default_interval(type)}
      else
        case Integer.parse(value) do
          {n, ""} when n > 0 -> {:ok, n}
          _ -> {:error, "#{Dashboard.task_type_label(type)}: interval must be a positive number of seconds."}
        end
      end
    else
      {:ok, 0}
    end
  end

  defp task_spec(type, target, _port, interval) when type in ["trace", "tracelb"] do
    {%{"target" => target, "method" => "icmp-paris"},
     %{schedule_enabled: true, schedule_interval_seconds: interval, schedule_jitter_percent: 10}}
  end

  defp task_spec("ping", target, _port, interval) do
    {%{"target" => target, "count" => 5, "size" => 64},
     %{schedule_enabled: true, schedule_interval_seconds: interval, schedule_jitter_percent: 10}}
  end

  defp task_spec("stamp-sender", target, port, interval) do
    {%{
       "target" => target,
       "target_port" => port,
       "dscp" => 0,
       "require_clock_sync" => false,
       "packet_count" => 20,
       "packet_interval" => 100_000_000
     },
     %{schedule_enabled: true, schedule_interval_seconds: interval, schedule_jitter_percent: 10}}
  end

  defp task_spec("stamp-reflector", _target, port, _interval) do
    {%{"listen_port" => port, "dscp" => 0, "require_clock_sync" => false},
     %{schedule_enabled: false, schedule_interval_seconds: 0, schedule_jitter_percent: 0}}
  end

  defp unique_task_id(type, target, port, used_ids) do
    base =
      case type do
        "stamp-reflector" -> "stamp-reflector-#{port}"
        _ -> "#{type}-#{sanitize_target(target)}"
      end

    ensure_unique(base, used_ids, 1)
  end

  defp ensure_unique(base, used_ids, n) do
    candidate = if n == 1, do: base, else: "#{base}-#{n}"
    if MapSet.member?(used_ids, candidate), do: ensure_unique(base, used_ids, n + 1), else: candidate
  end

  defp sanitize_target(target) do
    target
    |> String.downcase()
    |> String.replace(~r/[^a-z0-9]+/, "-")
    |> String.trim("-")
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
