defmodule Cepheus.Dashboard do
  alias Cepheus.Repo

  # window key => {sql interval literal, time_bucket size for charts}
  @windows %{
    "15m" => {"15 minutes", "15 seconds"},
    "1h" => {"1 hour", "2 minutes"},
    "6h" => {"6 hours", "10 minutes"},
    "24h" => {"24 hours", "30 minutes"}
  }

  # Display order for the window switch (Map.keys/1 is unordered).
  @window_order ["15m", "1h", "6h", "24h"]

  def windows, do: @window_order

  def default_window, do: "15m"

  def valid_window?(w), do: Map.has_key?(@windows, w)

  defp window_interval!(w) do
    case Map.fetch(@windows, w) do
      {:ok, {interval, _}} -> interval
      :error -> raise ArgumentError, "invalid window: #{inspect(w)}"
    end
  end

  defp window_bucket!(w) do
    case Map.fetch(@windows, w) do
      {:ok, {_, bucket}} -> bucket
      :error -> raise ArgumentError, "invalid window: #{inspect(w)}"
    end
  end

  def list_devices do
    sql = """
    SELECT d.serial_id,
           d.agent_config_id::text,
           ac.generation,
           ac.report_endpoint,
           ac.report_interval_seconds,
           ac.scamper_pps,
           ac.created_at
    FROM device d
    JOIN agent_config ac ON ac.id = d.agent_config_id
    ORDER BY d.serial_id
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])

    Enum.map(rows, fn [serial_id, config_id, gen, endpoint, interval, pps, created_at] ->
      %{
        serial_id: serial_id,
        agent_config_id: config_id,
        generation: gen,
        report_endpoint: endpoint,
        report_interval_seconds: interval,
        scamper_pps: pps,
        created_at: created_at
      }
    end)
  end

  def get_device(serial_id) when is_binary(serial_id) do
    Enum.find(list_devices(), &(&1.serial_id == serial_id))
  end

  def list_tasks_for_config(agent_config_id) when is_binary(agent_config_id) do
    uuid_bin = Ecto.UUID.dump!(agent_config_id)

    sql = """
    SELECT id::text,
           task_id,
           type,
           enabled,
           schedule_interval_seconds,
           schedule_jitter_percent,
           schedule_enabled,
           params
    FROM agent_task
    WHERE agent_config_id = $1
    ORDER BY task_id
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [uuid_bin])

    Enum.map(rows, fn [id, task_id, type, enabled, interval, jitter, sched_enabled, params] ->
      %{
        id: id,
        task_id: task_id,
        type: type,
        enabled: enabled,
        schedule_interval_seconds: interval,
        schedule_jitter_percent: jitter,
        schedule_enabled: sched_enabled,
        params: decode_params(params)
      }
    end)
  end

  # ---------------------------------------------------------------------------
  # Probe task authoring (frontend task editor)
  # ---------------------------------------------------------------------------

  @doc "Task types selectable in the UI, as `{label, value}` pairs."
  def task_types do
    [
      {"Traceroute", "trace"},
      {"Ping", "ping"},
      {"STAMP sender", "stamp-sender"},
      {"STAMP reflector", "stamp-reflector"}
    ]
  end

  def task_type_label(type),
    do: Enum.find_value(task_types(), type, fn {label, value} -> value == type && label end)

  def task_has_target?(type), do: type in ["trace", "tracelb", "ping", "stamp-sender"]
  def task_has_port?(type), do: type in ["stamp-sender", "stamp-reflector"]
  def task_schedulable?(type), do: type != "stamp-reflector"

  def port_label("stamp-reflector"), do: "listen port"
  def port_label(_), do: "port"

  def default_interval("trace"), do: 40
  def default_interval("tracelb"), do: 40
  def default_interval("ping"), do: 20
  def default_interval("stamp-sender"), do: 10
  def default_interval(_), do: 30

  def default_port(type), do: if(task_has_port?(type), do: "862", else: "")

  @doc """
  Distinct traceroute targets (among the queued `rows` plus the config's
  `existing_tasks`) that have no companion ping/STAMP-sender probe to the same
  IP. Traceroute needs a latency probe to the same target, so the UI uses this
  to nudge the user.
  """
  def recommended_trace_targets(rows, existing_tasks) do
    covered =
      (Enum.map(rows, fn r -> {r.type, String.trim(r.target || "")} end) ++
         Enum.map(existing_tasks, fn t -> {t.type, Map.get(t.params || %{}, "target", "")} end))
      |> Enum.filter(fn {type, target} -> type in ["ping", "stamp-sender"] and target != "" end)
      |> MapSet.new(fn {_type, target} -> target end)

    rows
    |> Enum.filter(fn r -> r.type in ["trace", "tracelb"] end)
    |> Enum.map(fn r -> String.trim(r.target || "") end)
    |> Enum.reject(fn target -> target == "" or MapSet.member?(covered, target) end)
    |> Enum.uniq()
  end

  @doc """
  Inserts new agent tasks for a config and bumps the config generation (so the
  agent reconciles on its next pull). Runs in a single transaction.

  `tasks` is a list of maps with keys `:task_id`, `:type`, `:params` (a map),
  `:schedule_enabled`, `:schedule_interval_seconds`, `:schedule_jitter_percent`.
  Returns `{:ok, new_generation}` or `{:error, message}`.
  """
  def add_tasks(agent_config_id, tasks) when is_binary(agent_config_id) and is_list(tasks) do
    uuid = Ecto.UUID.dump!(agent_config_id)

    Repo.transaction(fn ->
      %Postgrex.Result{rows: [[generation]]} =
        Ecto.Adapters.SQL.query!(
          Repo,
          "UPDATE agent_config SET generation = generation + 1, updated_at = NOW() WHERE id = $1 RETURNING generation",
          [uuid]
        )

      Enum.each(tasks, fn t ->
        Ecto.Adapters.SQL.query!(
          Repo,
          """
          INSERT INTO agent_task
            (agent_config_id, task_id, type, enabled, generation,
             schedule_interval_seconds, schedule_jitter_percent, schedule_enabled, params)
          VALUES ($1, $2, $3, true, $4, $5, $6, $7, $8::jsonb)
          """,
          [
            uuid,
            t.task_id,
            t.type,
            generation,
            t.schedule_interval_seconds,
            t.schedule_jitter_percent,
            t.schedule_enabled,
            t.params
          ]
        )
      end)

      generation
    end)
    |> case do
      {:ok, generation} -> {:ok, generation}
      {:error, reason} -> {:error, inspect(reason)}
    end
  rescue
    e -> {:error, Exception.message(e)}
  end

  # jsonb columns come back from raw SQL as JSON strings here; normalize to a map.
  defp decode_params(params) when is_map(params), do: params

  defp decode_params(params) when is_binary(params) do
    case Jason.decode(params) do
      {:ok, map} when is_map(map) -> map
      _ -> %{}
    end
  end

  defp decode_params(_), do: %{}

  @doc """
  Per-target rollup over the window, read from `stamp_measurements` only.

  Uses the precomputed p95 columns (`rtt_p95_ns`, `fwd_p95_ns`, `bwd_p95_ns`) on
  `stamp_measurements`. Does NOT touch the `stamp_probes` hypertable.

  When `targets` is `nil` (default) every target the device has reported is
  included; pass a list of strings to restrict the result set.
  """
  def summary_by_target(serial_id, window, targets \\ nil)
      when is_binary(serial_id) and is_binary(window) and (is_list(targets) or is_nil(targets)) do
    interval = window_interval!(window)
    {target_clause, params} = target_filter_clause(targets, [serial_id], "m.target")

    sql = """
    SELECT m.target,
           SUM(m.sent)::BIGINT                                 AS sent,
           SUM(m.received)::BIGINT                             AS received,
           CASE WHEN SUM(m.sent) = 0 THEN NULL
                ELSE 1.0 - (SUM(m.received)::float / SUM(m.sent))
           END                                                 AS loss,
           AVG(m.rtt_p95_ns)::BIGINT                           AS avg_rtt_p95_ns,
           MAX(m.rtt_p95_ns)                                   AS max_rtt_p95_ns,
           AVG(m.fwd_p95_ns)::BIGINT                           AS avg_fwd_p95_ns,
           AVG(m.bwd_p95_ns)::BIGINT                           AS avg_bwd_p95_ns,
           COUNT(*)                                            AS measurements,
           MAX(m.timestamp)                                    AS last_seen
    FROM stamp_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY m.target
    ORDER BY m.target
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    Enum.map(rows, fn [
                        target,
                        sent,
                        received,
                        loss,
                        avg_rtt_p95,
                        max_rtt_p95,
                        avg_fwd_p95,
                        avg_bwd_p95,
                        measurements,
                        last_seen
                      ] ->
      %{
        target: target,
        sent: sent,
        received: received,
        loss: loss,
        avg_rtt_p95_ns: avg_rtt_p95,
        max_rtt_p95_ns: max_rtt_p95,
        avg_fwd_p95_ns: avg_fwd_p95,
        avg_bwd_p95_ns: avg_bwd_p95,
        measurements: measurements,
        last_seen: last_seen
      }
    end)
  end

  @doc """
  Time-bucketed RTT p95 series per target for the chart.

  Returns an ApexCharts-shaped list:
      [%{name: "RTT · target", data: [[unix_ms, rtt_p95_ns], ...]}, ...]

  Pass `targets` (list of strings) to restrict to a subset.
  """
  def measurement_timeseries(serial_id, window, targets \\ nil)
      when is_binary(serial_id) and is_binary(window) and (is_list(targets) or is_nil(targets)) do
    interval = window_interval!(window)
    bucket = window_bucket!(window)
    {target_clause, params} = target_filter_clause(targets, [serial_id], "m.target")

    sql = """
    SELECT time_bucket(INTERVAL '#{bucket}', m.timestamp) AS bucket,
           m.target,
           AVG(m.rtt_p95_ns)::BIGINT                      AS rtt_p95_ns,
           AVG(m.fwd_p95_ns)::BIGINT  AS fwd_p95_ns,
           AVG(m.bwd_p95_ns)::BIGINT  AS bwd_p95_ns,
           AVG(m.loss)::FLOAT         AS loss
    FROM stamp_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY bucket, m.target
    ORDER BY m.target, bucket
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    rows
    |> Enum.group_by(fn [_bucket, target, _rtt, _fwd, _bwd, _loss] -> target end)
    |> Enum.flat_map(fn {target, points} ->
      rtt_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, rtt, _fwd, _bwd, _loss] ->
          [DateTime.to_unix(bucket, :millisecond), rtt]
        end)

      fwd_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _rtt, fwd, _bwd, _loss] ->
          [DateTime.to_unix(bucket, :millisecond), fwd]
        end)

      bwd_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _rtt, _fwd, bwd, _loss] ->
          [DateTime.to_unix(bucket, :millisecond), bwd]
        end)

      loss_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _rtt, _fwd, _bwd, loss] ->
          [DateTime.to_unix(bucket, :millisecond), loss_pct(loss)]
        end)

      [
        %{name: "RTT · #{target}", data: rtt_data},
        %{name: "Forward · #{target}", data: fwd_data},
        %{name: "Backward · #{target}", data: bwd_data},
        %{name: "Loss % · #{target}", data: loss_data}
      ]
    end)
    |> Enum.sort_by(& &1.name)
  end

  # Loss is stored as a 0.0–1.0 fraction; the chart plots it as a percentage on
  # a dedicated secondary axis.
  defp loss_pct(nil), do: nil
  defp loss_pct(loss), do: Float.round(loss * 100, 2)

  @target_list_window "24 hours"

  @doc """
  Distinct STAMP targets seen for this device in the last 24 hours.

  Used to populate the target selector. The window is wider than the chart
  window so a target the user briefly stops probing doesn't disappear from
  the selector.
  """
  def list_stamp_targets(serial_id) when is_binary(serial_id) do
    sql = """
    SELECT DISTINCT target
    FROM stamp_measurements
    WHERE serial_id = $1
      AND timestamp > NOW() - INTERVAL '#{@target_list_window}'
    ORDER BY target
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])
    Enum.map(rows, fn [t] -> t end)
  end

  @doc """
  Per-target ping rollup over the window, read from `ping_measurements`.
  """
  def ping_summary_by_target(serial_id, window, targets \\ nil)
      when is_binary(serial_id) and is_binary(window) and (is_list(targets) or is_nil(targets)) do
    interval = window_interval!(window)
    {target_clause, params} = target_filter_clause(targets, [serial_id], "m.target")

    sql = """
    SELECT m.target,
           SUM(m.sent)::BIGINT                                 AS sent,
           SUM(m.received)::BIGINT                             AS received,
           CASE WHEN SUM(m.sent) = 0 THEN NULL
                ELSE 1.0 - (SUM(m.received)::float / SUM(m.sent))
           END                                                 AS loss,
           AVG(m.rtt_avg_ns)::BIGINT                           AS avg_rtt_ns,
           MIN(m.rtt_min_ns)                                   AS min_rtt_ns,
           MAX(m.rtt_max_ns)                                   AS max_rtt_ns,
           AVG(m.rtt_p95_ns)::BIGINT                           AS avg_p95_ns,
           COUNT(*)                                            AS measurements,
           MAX(m.timestamp)                                    AS last_seen
    FROM ping_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY m.target
    ORDER BY m.target
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    Enum.map(rows, fn [
                        target,
                        sent,
                        received,
                        loss,
                        avg_rtt,
                        min_rtt,
                        max_rtt,
                        avg_p95,
                        measurements,
                        last_seen
                      ] ->
      %{
        target: target,
        sent: sent,
        received: received,
        loss: loss,
        avg_rtt_ns: avg_rtt,
        min_rtt_ns: min_rtt,
        max_rtt_ns: max_rtt,
        avg_p95_ns: avg_p95,
        measurements: measurements,
        last_seen: last_seen
      }
    end)
  end

  @doc """
  Time-bucketed ping series per target. Returns ApexCharts shape with one
  series per target (avg RTT).
  """
  def ping_timeseries(serial_id, window, targets \\ nil)
      when is_binary(serial_id) and is_binary(window) and (is_list(targets) or is_nil(targets)) do
    interval = window_interval!(window)
    bucket = window_bucket!(window)
    {target_clause, params} = target_filter_clause(targets, [serial_id], "m.target")

    sql = """
    SELECT time_bucket(INTERVAL '#{bucket}', m.timestamp) AS bucket,
           m.target,
           AVG(m.rtt_avg_ns)::BIGINT                      AS rtt_avg_ns,
           AVG(m.rtt_p95_ns)::BIGINT                      AS rtt_p95_ns,
           AVG(m.loss)::FLOAT                             AS loss
    FROM ping_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY bucket, m.target
    ORDER BY m.target, bucket
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    rows
    |> Enum.group_by(fn [_bucket, target, _avg, _p95, _loss] -> target end)
    |> Enum.flat_map(fn {target, points} ->
      avg_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, avg, _p95, _loss] ->
          [DateTime.to_unix(bucket, :millisecond), avg]
        end)

      p95_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _avg, p95, _loss] ->
          [DateTime.to_unix(bucket, :millisecond), p95]
        end)

      loss_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _avg, _p95, loss] ->
          [DateTime.to_unix(bucket, :millisecond), loss_pct(loss)]
        end)

      [
        %{name: "avg · #{target}", data: avg_data},
        %{name: "p95 · #{target}", data: p95_data},
        %{name: "Loss % · #{target}", data: loss_data}
      ]
    end)
    |> Enum.sort_by(& &1.name)
  end

  @doc """
  Distinct ping targets seen for this device in the last 24 hours.
  """
  def list_ping_targets(serial_id) when is_binary(serial_id) do
    sql = """
    SELECT DISTINCT target
    FROM ping_measurements
    WHERE serial_id = $1
      AND timestamp > NOW() - INTERVAL '#{@target_list_window}'
    ORDER BY target
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])
    Enum.map(rows, fn [t] -> t end)
  end

  # Builds a SQL fragment + parameter list for an optional `target IN (...)` filter.
  # Returns `{"", params}` when no filter is applied so the caller can splice the
  # fragment after the static WHERE clauses.
  defp target_filter_clause(nil, params, _col), do: {"", params}

  defp target_filter_clause(targets, params, col) when is_list(targets) do
    case targets do
      [] ->
        # An empty selection means "no targets" — force zero rows.
        {"AND FALSE", params}

      _ ->
        idx = length(params) + 1
        {"AND #{col} = ANY($#{idx}::text[])", params ++ [targets]}
    end
  end

  @doc """
  Recent argus events for a device. Returns currently-open events plus any
  events opened within the selected window. Sorted open-first, then newest.
  """
  def events_for_device(serial_id, window) when is_binary(serial_id) and is_binary(window) do
    interval = window_interval!(window)

    # Join with argus_policy_state to surface the live policy status (clean /
    # watching / firing / recovering). The PK on policy_state includes detector,
    # so a (serial_id, target, port, metric) key may have multiple rows — we
    # take the worst (firing > recovering > watching > clean) so a single
    # detector still firing dominates the displayed status.
    sql = """
    WITH worst_policy AS (
      SELECT serial_id, target, port, metric,
             CASE
               WHEN bool_or(status = 'firing')     THEN 'firing'
               WHEN bool_or(status = 'recovering') THEN 'recovering'
               WHEN bool_or(status = 'watching')   THEN 'watching'
               ELSE 'clean'
             END AS policy_status
      FROM argus_policy_state
      GROUP BY serial_id, target, port, metric
    )
    SELECT e.id::text,
           e.target,
           e.port,
           e.metric,
           e.status,
           COALESCE(p.policy_status, 'unknown') AS policy_status,
           e.opened_at,
           e.last_seen_at,
           e.closed_at,
           e.finding_count,
           e.peak_severity,
           e.detectors
    FROM argus_events e
    LEFT JOIN worst_policy p
      ON p.serial_id = e.serial_id
     AND p.target    = e.target
     AND p.port      = e.port
     AND p.metric    = e.metric
    WHERE e.serial_id = $1
      AND (e.status = 'open' OR e.opened_at > NOW() - INTERVAL '#{interval}')
    ORDER BY
      CASE e.status WHEN 'open' THEN 0 ELSE 1 END,
      e.opened_at DESC
    LIMIT 50
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

    Enum.map(rows, fn [
                        id,
                        target,
                        port,
                        metric,
                        status,
                        policy_status,
                        opened_at,
                        last_seen_at,
                        closed_at,
                        finding_count,
                        peak_severity,
                        detectors
                      ] ->
      %{
        id: id,
        target: target,
        port: port,
        metric: metric,
        status: status,
        policy_status: policy_status,
        display_status: derive_display_status(status, policy_status),
        opened_at: opened_at,
        last_seen_at: last_seen_at,
        closed_at: closed_at,
        finding_count: finding_count,
        peak_severity: peak_severity,
        detectors: detectors || []
      }
    end)
  end

  defp derive_display_status("closed", _), do: "RESOLVED"
  defp derive_display_status("open", "recovering"), do: "RECOVERING"
  defp derive_display_status("open", _), do: "OPEN"
  defp derive_display_status(other, _), do: String.upcase(to_string(other))

  @doc """
  Most recent findings for a device, across all events.
  """
  def recent_findings_for_device(serial_id, window, limit \\ 50)
      when is_binary(serial_id) and is_binary(window) and is_integer(limit) do
    interval = window_interval!(window)

    sql = """
    SELECT id::text,
           target,
           port,
           metric,
           detector,
           ts,
           value,
           severity,
           details::text,
           event_id::text
    FROM argus_findings
    WHERE serial_id = $1
      AND ts > NOW() - INTERVAL '#{interval}'
    ORDER BY ts DESC
    LIMIT #{limit}
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

    Enum.map(rows, fn [
                        id,
                        target,
                        port,
                        metric,
                        detector,
                        ts,
                        value,
                        severity,
                        details,
                        event_id
                      ] ->
      %{
        id: id,
        target: target,
        port: port,
        metric: metric,
        detector: detector,
        ts: ts,
        value: value,
        severity: severity,
        details: details,
        event_id: event_id
      }
    end)
  end

  @doc """
  Map of serial_id → count of currently-open events. Used by the index page
  to badge devices that have active alerts.
  """
  def open_event_counts_by_device do
    sql = """
    SELECT serial_id, COUNT(*)::int
    FROM argus_events
    WHERE status = 'open'
    GROUP BY serial_id
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])
    Map.new(rows, fn [serial_id, count] -> {serial_id, count} end)
  end

  @doc "Total number of currently-open events across the whole fleet."
  def total_open_events do
    open_event_counts_by_device() |> Map.values() |> Enum.sum()
  end

  @doc """
  Per-device open-event stats: `serial_id => {count, max_peak_severity}`. Used to
  derive a fleet agent's health (alert vs degraded) on the overview page.
  """
  def open_event_stats_by_device do
    sql = """
    SELECT serial_id, COUNT(*)::int, MAX(peak_severity)
    FROM argus_events
    WHERE status = 'open'
    GROUP BY serial_id
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])
    Map.new(rows, fn [serial_id, count, max_peak] -> {serial_id, {count, max_peak}} end)
  end

  # ---------------------------------------------------------------------------
  # Fleet overview rollups
  # ---------------------------------------------------------------------------

  @doc """
  One row per agent for the fleet overview: target count, STAMP/Ping p95, worst
  loss, open-event count, a short RTT trend series (for the tick-strip) and a
  derived health status. Composed from the per-device rollups (a handful of
  queries per agent) — fine for the current fleet size.
  """
  def fleet_agents(window) when is_binary(window) do
    devices = list_devices()
    stats = open_event_stats_by_device()

    Enum.map(devices, fn d ->
      stamp = summary_by_target(d.serial_id, window)
      ping = ping_summary_by_target(d.serial_id, window)
      series = measurement_timeseries(d.serial_id, window)

      trend =
        series
        |> Enum.filter(&String.starts_with?(&1.name, "RTT ·"))
        |> Enum.flat_map(& &1.data)
        |> Enum.map(fn [_ts, v] -> v end)

      {open_count, max_peak} = Map.get(stats, d.serial_id, {0, nil})
      last_seen = latest_seen(stamp ++ ping)

      targets =
        (Enum.map(stamp, & &1.target) ++ Enum.map(ping, & &1.target)) |> Enum.uniq() |> length()

      %{
        serial_id: d.serial_id,
        region: nil,
        targets: targets,
        stamp_p95_ns: avg_ns(Enum.map(stamp, & &1.avg_rtt_p95_ns)),
        ping_p95_ns: avg_ns(Enum.map(ping, & &1.avg_p95_ns)),
        worst_loss: max_float(Enum.map(stamp, & &1.loss) ++ Enum.map(ping, & &1.loss)),
        open_events: open_count,
        trend: trend,
        status: agent_status(open_count, max_peak, last_seen, d.report_interval_seconds)
      }
    end)
  end

  @doc """
  Fleet-wide heartbeat KPIs derived from a `fleet_agents/1` result — no extra
  queries. Returns open events, STAMP p95, Ping p95 and worst loss.
  """
  def heartbeat_from_agents(agents) when is_list(agents) do
    %{
      open_events: agents |> Enum.map(& &1.open_events) |> Enum.sum(),
      stamp_p95_ns: avg_ns(Enum.map(agents, & &1.stamp_p95_ns)),
      ping_p95_ns: avg_ns(Enum.map(agents, & &1.ping_p95_ns)),
      worst_loss: max_float(Enum.map(agents, & &1.worst_loss))
    }
  end

  @doc """
  Detection-family summary tiles for the overview. Groups events (open + recent)
  by family (`stamp` | `ping` | `loss`) with open count, distinct agents and
  total findings. The `path` family is omitted (detection not implemented).
  """
  def family_stats(window) when is_binary(window) do
    interval = window_interval!(window)

    sql = """
    SELECT serial_id, metric, status, finding_count
    FROM argus_events
    WHERE status = 'open' OR opened_at > NOW() - INTERVAL '#{interval}'
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])

    acc =
      Enum.reduce(rows, %{}, fn [serial_id, metric, status, findings], acc ->
        family = metric_family(metric)
        f = Map.get(acc, family, %{open: 0, agents: MapSet.new(), findings: 0})

        f = %{
          open: f.open + if(status == "open", do: 1, else: 0),
          agents: MapSet.put(f.agents, serial_id),
          findings: f.findings + (findings || 0)
        }

        Map.put(acc, family, f)
      end)

    for {id, label, detector} <- [
          {"stamp", "STAMP latency", "EWMA"},
          {"ping", "Ping RTT", "EWMA"},
          {"loss", "Loss", "BETA-BINOMIAL"}
        ] do
      f = Map.get(acc, id, %{open: 0, agents: MapSet.new(), findings: 0})

      %{
        id: id,
        label: label,
        detector: detector,
        open: f.open,
        agents: MapSet.size(f.agents),
        findings: f.findings
      }
    end
  end

  @doc """
  Fleet-wide event list for the Alerts inbox: currently-open plus any opened in
  the window, newest first (open before resolved), capped at 200. Filtering by
  status/severity/text is done by the caller.
  """
  def all_events(window) when is_binary(window) do
    interval = window_interval!(window)

    sql = """
    SELECT id::text, serial_id, target, port, metric, status,
           opened_at, last_seen_at, closed_at, finding_count, peak_severity, detectors
    FROM argus_events
    WHERE status = 'open' OR opened_at > NOW() - INTERVAL '#{interval}'
    ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, opened_at DESC
    LIMIT 200
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])

    Enum.map(rows, fn [
                        id,
                        serial_id,
                        target,
                        port,
                        metric,
                        status,
                        opened_at,
                        last_seen_at,
                        closed_at,
                        finding_count,
                        peak_severity,
                        detectors
                      ] ->
      %{
        id: id,
        serial_id: serial_id,
        target: target,
        port: port,
        metric: metric,
        status: status,
        display_status: derive_display_status(status, nil),
        family: metric_family(metric),
        opened_at: opened_at,
        last_seen_at: last_seen_at,
        closed_at: closed_at,
        finding_count: finding_count,
        peak_severity: peak_severity,
        detectors: detectors || []
      }
    end)
  end

  @doc """
  A single event plus its recent findings (newest first), for the Alerts triage
  panel. Returns `nil` if the event id is missing or malformed.
  """
  def event_detail(event_id) when is_binary(event_id) do
    with {:ok, uuid} <- Ecto.UUID.dump(event_id) do
      event_sql = """
      SELECT id::text, serial_id, target, port, metric, status,
             opened_at, last_seen_at, closed_at, finding_count, peak_severity, detectors
      FROM argus_events
      WHERE id = $1
      """

      case Ecto.Adapters.SQL.query!(Repo, event_sql, [uuid]) do
        %Postgrex.Result{rows: []} ->
          nil

        %Postgrex.Result{rows: [row]} ->
          [id, serial_id, target, port, metric, status, opened_at, last_seen_at, closed_at,
           finding_count, peak_severity, detectors] = row

          findings_sql = """
          SELECT ts, value, severity, details::text
          FROM argus_findings
          WHERE event_id = $1
          ORDER BY ts DESC
          LIMIT 50
          """

          %Postgrex.Result{rows: frows} = Ecto.Adapters.SQL.query!(Repo, findings_sql, [uuid])

          findings =
            Enum.map(frows, fn [ts, value, severity, details] ->
              %{ts: ts, value: value, severity: severity, details: details}
            end)

          %{
            event: %{
              id: id,
              serial_id: serial_id,
              target: target,
              port: port,
              metric: metric,
              status: status,
              display_status: derive_display_status(status, nil),
              family: metric_family(metric),
              opened_at: opened_at,
              last_seen_at: last_seen_at,
              closed_at: closed_at,
              finding_count: finding_count,
              peak_severity: peak_severity,
              detectors: detectors || []
            },
            findings: findings
          }
      end
    else
      _ -> nil
    end
  end

  @doc """
  Top agent→target probe paths by STAMP p95 over the window, each with a small
  RTT spark for the tick-strip. Capped at `limit`.
  """
  def probe_paths(window, limit \\ 8) when is_binary(window) and is_integer(limit) do
    interval = window_interval!(window)

    sql = """
    SELECT serial_id, target, AVG(rtt_p95_ns)::bigint AS P95, MAX(timestamp) as last_seen
    FROM stamp_measurements
    WHERE timestamp > NOW() - interval '#{interval}'
    GROUP BY serial_id, target, port

    UNION ALL

    SELECT serial_id, target, AVG(rtt_p95_ns)::bigint AS P95, MAX(timestamp) as last_seen
    FROM ping_measurements
    WHERE timestamp > NOW() - interval '#{interval}'
    GROUP BY serial_id, target
    ORDER BY P95 DESC NULLS LAST
    LIMIT #{limit};

    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [])

    Enum.map(rows, fn [serial_id, target, p95, _last_seen] ->
      spark =
        serial_id
        |> measurement_timeseries(window, [target])
        |> Enum.filter(&String.starts_with?(&1.name, "RTT ·"))
        |> Enum.flat_map(& &1.data)
        |> Enum.map(fn [_ts, v] -> v end)

      %{serial_id: serial_id, target: target, p95_ns: p95, spark: spark}
    end)
  end

  @doc """
  Jeffreys-prior loss summary from aggregate sent/received counts: posterior-ish
  mean loss with a Wilson 95% interval, all as percentages. Returns `nil` when
  there were no probes. Closed-form (no stats dependency) — the credible-interval
  bar in the UI only needs a sane mean + lo/hi.
  """
  def loss_posterior(received, sent)
      when is_integer(received) and is_integer(sent) and sent > 0 do
    lost = max(sent - received, 0)
    n = sent
    p = lost / n
    z = 1.96
    denom = 1 + z * z / n
    center = (p + z * z / (2 * n)) / denom
    margin = z / denom * :math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))

    %{
      mean_pct: p * 100,
      lo_pct: max(center - margin, 0.0) * 100,
      hi_pct: min(center + margin, 1.0) * 100
    }
  end

  def loss_posterior(_received, _sent), do: nil

  @doc """
  Coarse detection family for an event/finding metric. Path is recognised but
  callers omit it (detection not implemented). The STAMP/Ping split is a
  best-effort string match since the stored metric may not always carry a
  series prefix.
  """
  def metric_family(metric) when is_binary(metric) do
    m = String.downcase(metric)

    cond do
      String.contains?(m, "loss") or String.contains?(m, "betabinom") -> "loss"
      String.contains?(m, ["path", "asn", "link", "fingerprint"]) -> "path"
      String.contains?(m, "ping") -> "ping"
      true -> "stamp"
    end
  end

  def metric_family(_), do: "stamp"

  # ----- small rollup helpers -----

  defp avg_ns(values) do
    case Enum.filter(values, &is_integer/1) do
      [] -> nil
      ns -> div(Enum.sum(ns), length(ns))
    end
  end

  defp max_float(values) do
    case Enum.filter(values, &is_float/1) do
      [] -> nil
      fs -> Enum.max(fs)
    end
  end

  defp latest_seen(rows) do
    rows
    |> Enum.map(& &1.last_seen)
    |> Enum.filter(& &1)
    |> case do
      [] -> nil
      stamps -> Enum.reduce(stamps, fn a, b -> if DateTime.compare(a, b) == :gt, do: a, else: b end)
    end
  end

  # Health rollup: alert when a high-severity event is open, degraded when any
  # event is open or the agent has gone stale, otherwise nominal.
  defp agent_status(open_count, max_peak, last_seen, report_interval_seconds) do
    stale =
      case last_seen do
        nil ->
          true

        dt ->
          DateTime.diff(DateTime.utc_now(), dt) > max((report_interval_seconds || 60) * 2, 60)
      end

    cond do
      open_count > 0 and is_float(max_peak) and max_peak >= 4.0 -> "alert"
      open_count > 0 -> "degraded"
      stale -> "degraded"
      true -> "nominal"
    end
  end
end
