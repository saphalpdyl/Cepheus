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
    SELECT target,
           AVG(avg_rtt)::BIGINT AS avg_rtt,
           SUM(sent)            AS total_sent,
           SUM(received)        AS total_received,
           AVG(loss)            AS avg_loss,
           COUNT(*)             AS samples,
           MAX(timestamp)       AS last_seen
    FROM stamp_data
    WHERE serial_id = $1
      AND timestamp > NOW() - INTERVAL '#{window}'
    GROUP BY target
    ORDER BY target
    """

    %Postgrex.Result{rows: rows} =
      Ecto.Adapters.SQL.query!(Repo, sql, [serial_id])

    Enum.map(rows, fn [target, avg_rtt, total_sent, total_received, avg_loss, samples, last_seen] ->
      %{
        target: target,
        avg_rtt_us: avg_rtt,
        avg_loss: avg_loss,
        samples: samples,
        last_seen: last_seen,
        total_sent: total_sent,
        total_received: total_received,
      }
    end)
  end
end
