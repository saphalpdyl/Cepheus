defmodule CepheusWeb.DeviceLive do
  @moduledoc """
  Agent detail — per-agent Tier-1 detection: STAMP/Ping latency by target,
  β-binomial loss, STAMP target rollups, detection events, per-sample findings,
  and the probe-task editor.
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
      |> assign(:chart_metric, "rtt")
      |> assign(:expanded_findings, MapSet.new())
      |> assign(:selected_stamp_targets, nil)
      |> assign(:selected_ping_targets, nil)
      |> assign(:task_rows, [])
      |> assign(:open_alerts_count, 0)

    {:ok, socket}
  end

  @impl true
  def handle_params(%{"serial_id" => serial_id}, _uri, socket) do
    case Dashboard.get_device(serial_id) do
      nil ->
        {:noreply,
         socket
         |> put_flash(:error, "Agent #{serial_id} not found")
         |> push_navigate(to: ~p"/")}

      device ->
        stamp_targets = Dashboard.list_stamp_targets(serial_id)
        chart_metric = if stamp_targets == [], do: "ping", else: "rtt"

        {:noreply,
         socket
         |> assign(:device, device)
         |> assign(:page_title, "Cepheus · #{device.serial_id}")
         |> assign(:chart_metric, chart_metric)
         |> load_show()}
    end
  end

  @impl true
  def handle_info(:tick, socket) do
    schedule_tick()
    {:noreply, load_show(socket)}
  end

  @impl true
  def handle_event("set_window", %{"window" => window}, socket) do
    if Dashboard.valid_window?(window),
      do: {:noreply, socket |> assign(:window, window) |> load_show()},
      else: {:noreply, socket}
  end

  def handle_event("refresh", _params, socket), do: {:noreply, load_show(socket)}

  def handle_event("set_chart_metric", %{"metric" => metric}, socket) when metric in ["rtt", "ping"] do
    {:noreply, socket |> assign(:chart_metric, metric) |> load_show()}
  end

  def handle_event("set_stamp_targets", params, socket) do
    selected = params |> Map.get("targets", []) |> List.wrap()
    {:noreply, socket |> assign(:selected_stamp_targets, selected) |> load_show()}
  end

  def handle_event("set_ping_targets", params, socket) do
    selected = params |> Map.get("targets", []) |> List.wrap()
    {:noreply, socket |> assign(:selected_ping_targets, selected) |> load_show()}
  end

  def handle_event("toggle_finding_group", %{"key" => key}, socket) do
    expanded = socket.assigns.expanded_findings

    expanded =
      if MapSet.member?(expanded, key),
        do: MapSet.delete(expanded, key),
        else: MapSet.put(expanded, key)

    {:noreply, assign(socket, :expanded_findings, expanded)}
  end

  # ----- Task editor (carried over from the previous dashboard) -----

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
      {:noreply,
       socket
       |> assign(:task_rows, [])
       |> assign(:device, Dashboard.get_device(device.serial_id))
       |> put_flash(:info, "Added #{length(tasks)} task(s) · agent now on generation #{generation}")
       |> load_show()}
    else
      {:error, message} -> {:noreply, put_flash(socket, :error, message)}
    end
  end

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

  defp load_show(socket) do
    %{
      device: device,
      window: window,
      chart_metric: chart_metric,
      selected_stamp_targets: sel_stamp,
      selected_ping_targets: sel_ping
    } = socket.assigns

    tasks = Dashboard.list_tasks_for_config(device.agent_config_id)
    stamp_targets = Dashboard.list_stamp_targets(device.serial_id)
    ping_targets = Dashboard.list_ping_targets(device.serial_id)

    summary = Dashboard.summary_by_target(device.serial_id, window)
    ping_summary = Dashboard.ping_summary_by_target(device.serial_id, window)
    events = Dashboard.events_for_device(device.serial_id, window)
    findings = Dashboard.recent_findings_for_device(device.serial_id, window)

    {loss_rows, loss_cap} = build_loss_rows(summary, ping_summary)

    socket
    |> assign(:tasks, tasks)
    |> assign(:stamp_targets, stamp_targets)
    |> assign(:ping_targets, ping_targets)
    |> assign(:summary, summary)
    |> assign(:ping_summary, ping_summary)
    |> assign(:ping_by_target, Map.new(ping_summary, &{&1.target, &1}))
    |> assign(:vitals, compute_vitals(summary, ping_summary, events))
    |> assign(:loss_rows, loss_rows)
    |> assign(:loss_cap, loss_cap)
    |> assign(:events, events)
    |> assign(:findings, findings)
    |> assign(:open_alerts_count, Dashboard.total_open_events())
    |> push_event("latency-chart:update", %{
      series: chart_series(device.serial_id, window, chart_metric, sel_stamp, sel_ping)
    })
  end

  # Build the one-line-per-target series for the latency chart, depending on the
  # selected metric (STAMP RTT p95 vs Ping RTT p95). Names are stripped to just
  # the target so the legend reads cleanly.
  defp chart_series(serial_id, window, "ping", _sel_stamp, sel_ping) do
    serial_id
    |> Dashboard.ping_timeseries(window, sel_ping)
    |> Enum.filter(&String.starts_with?(&1.name, "p95 · "))
    |> Enum.map(&strip_prefix(&1, "p95 · "))
  end

  defp chart_series(serial_id, window, _rtt, sel_stamp, _sel_ping) do
    serial_id
    |> Dashboard.measurement_timeseries(window, sel_stamp)
    |> Enum.filter(&String.starts_with?(&1.name, "RTT · "))
    |> Enum.map(&strip_prefix(&1, "RTT · "))
  end

  defp strip_prefix(%{name: name} = series, prefix),
    do: %{series | name: String.replace_prefix(name, prefix, "")}

  defp compute_vitals(summary, ping_summary, events) do
    targets =
      (Enum.map(summary, & &1.target) ++ Enum.map(ping_summary, & &1.target))
      |> Enum.uniq()
      |> length()

    %{
      targets: targets,
      stamp_p95_ns: avg_ns(Enum.map(summary, & &1.avg_rtt_p95_ns)),
      ping_p95_ns: avg_ns(Enum.map(ping_summary, & &1.avg_p95_ns)),
      worst_loss: max_loss(Enum.map(summary, & &1.loss) ++ Enum.map(ping_summary, & &1.loss)),
      open_events: Enum.count(events, &(&1.status == "open"))
    }
  end

  # One row per target with STAMP + Ping loss posteriors, plus a shared bar cap.
  defp build_loss_rows(summary, ping_summary) do
    stamp_by = Map.new(summary, &{&1.target, &1})
    ping_by = Map.new(ping_summary, &{&1.target, &1})

    targets =
      (Enum.map(summary, & &1.target) ++ Enum.map(ping_summary, & &1.target))
      |> Enum.uniq()
      |> Enum.sort()

    rows =
      Enum.map(targets, fn target ->
        stamp = Map.get(stamp_by, target)
        ping = Map.get(ping_by, target)

        %{
          target: target,
          stamp: stamp,
          ping: ping,
          stamp_model: model_for(stamp),
          ping_model: model_for(ping)
        }
      end)

    cap =
      rows
      |> Enum.flat_map(fn r -> [r.stamp_model, r.ping_model] end)
      |> Enum.filter(& &1)
      |> Enum.map(& &1.hi_pct)
      |> case do
        [] -> 0.6
        his -> max(0.6, Enum.max(his) * 1.1)
      end

    {rows, cap}
  end

  defp model_for(nil), do: nil

  defp model_for(%{received: received, sent: sent})
       when is_integer(received) and is_integer(sent),
       do: Dashboard.loss_posterior(received, sent)

  defp model_for(_), do: nil

  defp avg_ns(values) do
    case Enum.filter(values, &is_integer/1) do
      [] -> nil
      ns -> div(Enum.sum(ns), length(ns))
    end
  end

  defp max_loss(values) do
    case Enum.filter(values, &is_float/1) do
      [] -> nil
      fs -> Enum.max(fs)
    end
  end

  # ----- task building / validation (verbatim from the previous dashboard) -----

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
end
