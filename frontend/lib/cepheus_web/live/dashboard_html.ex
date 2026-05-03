defmodule CepheusWeb.DashboardHTML do
  @moduledoc """
  Function components and presentation helpers for `CepheusWeb.DashboardLive`.

  Page-sized templates (`index_view`, `show_view`) live as `.heex` files in
  `dashboard_html/` and are compiled into functions on this module via
  `embed_templates/1`. Small reusable components (`toolbar`) and presentation
  helpers (`fmt_rtt`, `device_online?`, …) stay here.
  """
  use CepheusWeb, :html

  embed_templates "dashboard_html/*"

  attr :window, :string, required: true
  attr :windows, :list, required: true
  attr :last_loaded_at, :any, required: true

  def toolbar(assigns) do
    ~H"""
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-xs text-base-content/60">
        Last refreshed {fmt_dt(@last_loaded_at)} · auto-refresh every 2s
      </p>
      <form phx-change="set_window" class="flex items-center gap-2">
        <label for="window" class="text-xs text-base-content/70">Window</label>
        <select id="window" name="window" class="select select-bordered select-sm">
          <option :for={w <- @windows} value={w} selected={w == @window}>{w}</option>
        </select>
      </form>
    </div>
    """
  end

  def short_uuid(nil), do: ""
  def short_uuid(uuid) when is_binary(uuid), do: String.slice(uuid, 0, 8) <> "…"

  def fmt_dt(%DateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S UTC")
  def fmt_dt(%NaiveDateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")
  def fmt_dt(nil), do: "—"
  def fmt_dt(other), do: to_string(other)

  def fmt_loss(nil), do: "—"

  def fmt_loss(f) when is_float(f),
    do: :io_lib.format("~.2f", [f * 100]) |> IO.iodata_to_binary() |> Kernel.<>("%")

  def fmt_loss(other), do: to_string(other)

  def fmt_rtt(nil), do: "—"

  def fmt_rtt(ns) when is_integer(ns) do
    ms = ns / 1_000_000
    :io_lib.format("~.2f ms", [ms]) |> IO.iodata_to_binary()
  end

  def fmt_rtt(other), do: to_string(other)

  @doc """
  A device is considered "online" if its most recent measurement across all
  targets is within 2× its report interval.
  """
  def device_online?(device, summary) do
    threshold_seconds = max(device.report_interval_seconds * 2, 30)

    case summary |> Enum.map(& &1.last_seen) |> Enum.filter(& &1) do
      [] ->
        false

      stamps ->
        latest = Enum.reduce(stamps, &latest_dt/2)
        DateTime.diff(DateTime.utc_now(), latest) <= threshold_seconds
    end
  end

  defp latest_dt(a, b), do: if(DateTime.compare(a, b) == :gt, do: a, else: b)

  def summary_avg_p95([]), do: "—"

  def summary_avg_p95(summary) do
    case Enum.filter(summary, & &1.avg_rtt_p95_ns) do
      [] -> "—"
      rows -> rows |> Enum.map(& &1.avg_rtt_p95_ns) |> Enum.sum() |> div(length(rows)) |> fmt_rtt()
    end
  end

  def summary_worst_loss([]), do: "—"

  def summary_worst_loss(summary) do
    case Enum.filter(summary, & &1.loss) do
      [] -> "—"
      rows -> rows |> Enum.map(& &1.loss) |> Enum.max() |> fmt_loss()
    end
  end
end
