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
  """
  def summary_by_target(serial_id, window) when is_binary(serial_id) and is_binary(window) do
    interval = window_interval!(window)

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
    GROUP BY m.target
    ORDER BY m.target
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

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
      [%{name: target, data: [[unix_ms, rtt_p95_ns], ...]}, ...]
  """
  def measurement_timeseries(serial_id, window) when is_binary(serial_id) and is_binary(window) do
    interval = window_interval!(window)
    bucket = window_bucket!(window)

    sql = """
    SELECT time_bucket(INTERVAL '#{bucket}', m.timestamp) AS bucket,
           m.target,
           AVG(m.rtt_p95_ns)::BIGINT                      AS rtt_p95_ns,
           AVG(m.fwd_p95_ns)::BIGINT  AS fwd_p95_ns,
           AVG(m.bwd_p95_ns)::BIGINT  AS bwd_p95_ns
    FROM stamp_measurements m
    WHERE m.serial_id = $1
      AND m.timestamp > NOW() - INTERVAL '#{interval}'
    GROUP BY bucket, m.target
    ORDER BY m.target, bucket
    """

    %Postgrex.Result{rows: rows} = Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

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
        %{name: "RTT", data: rtt_data},
        %{name: "Forward", data: fwd_data},
        %{name: "Backward", data: bwd_data}
      ]
    end)
    |> Enum.sort_by(& &1.name)
  end
end
