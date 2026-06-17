defmodule CepheusWeb.AgentsLive do
  use CepheusWeb, :live_view

  alias Cepheus.Dashboard

  @refresh_ms 2000

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket), do: schedule_tick()

    socket =
      socket
      |> assign(:window, Dashboard.default_window())
      |> assign(:windows, Dashboard.windows())
      |> assign(:page_title, "Cepheus · Agents")
      |> assign(:query, "")
      |> load()

      {:ok, socket}
  end

  @impl true
  def handle_info(:tick, socket) do
    schedule_tick()
    {:noreply, load(socket)}
  end

  def load(socket) do
    %{
      query: q,
      window: window,
    } = socket.assigns

    agents = Dashboard.fleet_agents(window)
    filtered_agents = apply_search_filters(agents, q)

    socket
    |> assign(:agents, agents)
    |> assign(:filtered, filtered_agents)

  end

  @impl true
  def handle_event("search", %{"q" => q}, socket),
    do: {:noreply, socket |> assign(:query, q) |> load()}

  defp apply_search_filters(agents, q) do
    qn = q |> to_string() |> String.downcase() |> String.trim()

    Enum.filter(agents, fn a ->
      query_ok?(a, qn)
    end)
  end

  defp query_ok?(_e, ""), do: true
  defp query_ok?(a, q) do
    String.contains?(String.downcase(a.serial_id), q)
  end

  defp schedule_tick, do: Process.send_after(self(), :tick, @refresh_ms)

end
