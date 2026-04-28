defmodule Cepheus.Dashboard do
  alias Cepheus.Repo

  @windows %{
    "1 minute" => "1 minute",
    "5 minutes" => "5 minutes",
    "15 minutes" => "15 minutes"
  }

  def windows, do: Map.keys(@windows)

  def valid_window?(w), do: Map.has_key?(@windows, w)

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

  def avg_rtt_by_target(serial_id, window) when is_binary(serial_id) and is_binary(window) do
    unless valid_window?(window) do
      raise ArgumentError, "invalid window: #{inspect(window)}"
    end

    # Window is interpolated as a literal — guarded by valid_window?/1 above.
    sql = """
    SELECT m.target,
           AVG(p.rtt) FILTER (WHERE NOT p.is_lost)::BIGINT       AS avg_rtt,
           MIN(p.rtt) FILTER (WHERE NOT p.is_lost)               AS min_rtt,
           MAX(p.rtt) FILTER (WHERE NOT p.is_lost)               AS max_rtt,
           PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY p.rtt)
             FILTER (WHERE NOT p.is_lost)                        AS p50_rtt,
           PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY p.rtt)
             FILTER (WHERE NOT p.is_lost)                        AS p95_rtt,
           COUNT(*)                                              AS total_probes,
           COUNT(*) FILTER (WHERE NOT p.is_lost)                 AS received_probes,
           (COUNT(*) FILTER (WHERE p.is_lost))::float
             / NULLIF(COUNT(*), 0)                               AS loss,
           COUNT(DISTINCT m.id)                                  AS measurements,
           MAX(p.tx)                                             AS last_seen
    FROM stamp_measurements m
    JOIN stamp_probes p ON p.measurement_id = m.id
    WHERE m.serial_id = $1
      AND p.tx > NOW() - INTERVAL '#{window}'
    GROUP BY m.target
    ORDER BY m.target
    """

    %Postgrex.Result{rows: rows} =
      Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

    Enum.map(rows, fn [
                        target,
                        avg_rtt,
                        min_rtt,
                        max_rtt,
                        p50_rtt,
                        p95_rtt,
                        total_probes,
                        received_probes,
                        loss,
                        measurements,
                        last_seen
                      ] ->
      %{
        target: target,
        avg_rtt_ns: avg_rtt,
        min_rtt_ns: min_rtt,
        max_rtt_ns: max_rtt,
        p50_rtt_ns: trunc_or_nil(p50_rtt),
        p95_rtt_ns: trunc_or_nil(p95_rtt),
        total_probes: total_probes,
        received_probes: received_probes,
        loss: loss,
        measurements: measurements,
        last_seen: last_seen
      }
    end)
  end

  defp trunc_or_nil(nil), do: nil
  defp trunc_or_nil(n) when is_float(n), do: trunc(n)
  defp trunc_or_nil(n), do: n
end
