defmodule Cepheus.Dashboard do
  alias Cepheus.Repo

  # window => {sql interval literal, time_bucket size for charts}
  @windows %{
    "1 minute" => {"1 minute", "5 seconds"},
    "5 minutes" => {"5 minutes", "15 seconds"},
    "15 minutes" => {"15 minutes", "30 seconds"},
    "1 hour" => {"1 hour", "2 minutes"},
    "6 hours" => {"6 hours", "10 minutes"}
  }

  def windows, do: Map.keys(@windows)

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
        params: params
      }
    end)
  end

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
           AVG(m.bwd_p95_ns)::BIGINT  AS bwd_p95_ns
    FROM stamp_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY bucket, m.target
    ORDER BY m.target, bucket
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    rows
    |> Enum.group_by(fn [_bucket, target, _rtt, _fwd, _bwd] -> target end)
    |> Enum.flat_map(fn {target, points} ->
      rtt_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, rtt, _fwd, _bwd] ->
          [DateTime.to_unix(bucket, :millisecond), rtt]
        end)

      fwd_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _rtt, fwd, _bwd] ->
          [DateTime.to_unix(bucket, :millisecond), fwd]
        end)

      bwd_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _rtt, _fwd, bwd] ->
          [DateTime.to_unix(bucket, :millisecond), bwd]
        end)

      [
        %{name: "RTT · #{target}", data: rtt_data},
        %{name: "Forward · #{target}", data: fwd_data},
        %{name: "Backward · #{target}", data: bwd_data}
      ]
    end)
    |> Enum.sort_by(& &1.name)
  end

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
           AVG(m.rtt_p95_ns)::BIGINT                      AS rtt_p95_ns
    FROM ping_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
      #{target_clause}
    GROUP BY bucket, m.target
    ORDER BY m.target, bucket
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, params)

    rows
    |> Enum.group_by(fn [_bucket, target, _avg, _p95] -> target end)
    |> Enum.flat_map(fn {target, points} ->
      avg_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, avg, _p95] ->
          [DateTime.to_unix(bucket, :millisecond), avg]
        end)

      p95_data =
        Enum.map(points, fn [%DateTime{} = bucket, _target, _avg, p95] ->
          [DateTime.to_unix(bucket, :millisecond), p95]
        end)

      [
        %{name: "avg · #{target}", data: avg_data},
        %{name: "p95 · #{target}", data: p95_data}
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
end
