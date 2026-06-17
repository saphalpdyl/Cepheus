defmodule CepheusWeb.Layouts do
  @moduledoc """
  This module holds layouts and related functionality
  used by your application.
  """
  use CepheusWeb, :html

  # Embed all files in layouts/* within this module.
  # The default root.html.heex file contains the HTML
  # skeleton of your application, namely HTML headers
  # and other static content.
  embed_templates "layouts/*"

  @doc """
  Renders your app layout.

  This function is typically invoked from every template,
  and it often contains your application menu, sidebar,
  or similar.

  ## Examples

      <Layouts.app flash={@flash}>
        <h1>Content</h1>
      </Layouts.app>

  """
  attr :flash, :map, required: true, doc: "the map of flash messages"
  attr :active_nav, :atom, default: :overview, doc: "active sidebar item: :overview | :alerts | :findings"
  attr :open_alerts_count, :integer, default: 0
  attr :show_window, :boolean, default: true
  attr :window, :string, default: "15m"
  attr :windows, :list, default: []
  slot :breadcrumb, doc: "topbar breadcrumb content"
  slot :inner_block, required: true

  def app(assigns) do
    ~H"""
    <div class="grid min-h-screen grid-cols-[230px_1fr] max-[1180px]:grid-cols-1">
      <aside class="sticky top-0 flex h-screen flex-col overflow-y-auto border-r border-sl-200 bg-paper max-[1180px]:static max-[1180px]:h-auto">
        <div class="flex items-center gap-2 border-b border-sl-200 px-3 py-3">
          <.cepheus_mark size={200} />
        </div>

        <nav class="flex flex-col gap-px px-2 pb-4 mt-2">
          <.nav_item navigate={~p"/"} icon="hero-squares-2x2" active={@active_nav == :overview}>
            Overview
          </.nav_item>

          <div class="mt-3 flex items-center gap-1.5 px-2 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wide text-sl-500">
            <.icon name="hero-chart-bar" class="size-3" /> Insights
          </div>
          <.nav_item navigate={~p"/alerts"} icon="hero-exclamation-triangle" active={@active_nav == :alerts} count={@open_alerts_count}>
            Alerts
          </.nav_item>
          <.nav_item navigate={~p"/agents"} icon="hero-cpu-chip">
            Agents
          </.nav_item>
        </nav>
      </aside>

      <div class="flex min-w-0 flex-col">
        <header class="sticky top-0 z-20 flex min-h-[48px] items-center gap-3 border-b border-sl-200 bg-paper px-5">
          <nav class="flex items-center gap-2 text-sm text-sl-500">{render_slot(@breadcrumb)}</nav>
          <div class="flex-1"></div>
          <div :if={@show_window} class="flex items-center gap-2">
            <span class="text-xs text-sl-500">Window</span>
            <.window_switch window={@window} windows={@windows} />
          </div>
          <div class="inline-flex items-center gap-1.5 text-xs text-sl-500">
            <span class="live-dot relative size-1.5 rounded-full bg-seu-500"></span> Live
          </div>
          <button
            type="button"
            phx-click="refresh"
            class="inline-flex items-center gap-1.5 rounded-[5px] border border-sl-200 bg-paper px-2 py-1 text-xs text-sl-600 hover:bg-canvas-alt"
          >
            <.icon name="hero-arrow-path" class="size-3.5 text-sl-400" /> Refresh
          </button>
        </header>

        <main class="dotted-canvas flex flex-1 flex-col gap-4 px-5 py-4">
          {render_slot(@inner_block)}
        </main>
      </div>
    </div>

    <.flash_group flash={@flash} />
    """
  end

  attr :navigate, :string, required: true
  attr :icon, :string, required: true
  attr :active, :boolean, default: false
  attr :count, :integer, default: nil
  slot :inner_block, required: true

  defp nav_item(assigns) do
    ~H"""
    <.link
      navigate={@navigate}
      class={[
        "flex items-center gap-2.5 rounded-[5px] px-2 py-1.5 text-sm",
        @active && "bg-encoder-50 font-medium text-encoder-700" || "text-sl-600 hover:bg-canvas-alt hover:text-ink"
      ]}
    >
      <.icon name={@icon} class={["size-4", @active && "text-encoder-600" || "text-sl-400"]} />
      {render_slot(@inner_block)}
      <span
        :if={@count != nil}
        class={[
          "ml-auto rounded-full px-1.5 font-mono text-[10px] font-semibold",
          @count > 0 && "bg-leak-100 text-leak-700" || "bg-sl-100 text-sl-400"
        ]}
      >
        {@count}
      </span>
    </.link>
    """
  end

  @doc """
  Shows the flash group with standard titles and content.

  ## Examples

      <.flash_group flash={@flash} />
  """
  attr :flash, :map, required: true, doc: "the map of flash messages"
  attr :id, :string, default: "flash-group", doc: "the optional id of flash container"

  def flash_group(assigns) do
    ~H"""
    <div id={@id} aria-live="polite">
      <.flash kind={:info} flash={@flash} />
      <.flash kind={:error} flash={@flash} />

      <.flash
        id="client-error"
        kind={:error}
        title="We can't find the internet"
        phx-disconnected={show(".phx-client-error #client-error") |> JS.remove_attribute("hidden")}
        phx-connected={hide("#client-error") |> JS.set_attribute({"hidden", ""})}
        hidden
      >
        Attempting to reconnect
        <.icon name="hero-arrow-path" class="ml-1 size-3 motion-safe:animate-spin" />
      </.flash>

      <.flash
        id="server-error"
        kind={:error}
        title="Something went wrong!"
        phx-disconnected={show(".phx-server-error #server-error") |> JS.remove_attribute("hidden")}
        phx-connected={hide("#server-error") |> JS.set_attribute({"hidden", ""})}
        hidden
      >
        Attempting to reconnect
        <.icon name="hero-arrow-path" class="ml-1 size-3 motion-safe:animate-spin" />
      </.flash>
    </div>
    """
  end

  @doc """
  Provides dark vs light theme toggle based on themes defined in app.css.

  See <head> in root.html.heex which applies the theme before page load.
  """
  def theme_toggle(assigns) do
    ~H"""
    <div class="card relative flex flex-row items-center border-2 border-base-300 bg-base-300 rounded-full">
      <div class="absolute w-1/3 h-full rounded-full border-1 border-base-200 bg-base-100 brightness-200 left-0 [[data-theme=light]_&]:left-1/3 [[data-theme=dark]_&]:left-2/3 transition-[left]" />

      <button
        class="flex p-2 cursor-pointer w-1/3"
        phx-click={JS.dispatch("phx:set-theme")}
        data-phx-theme="system"
      >
        <.icon name="hero-computer-desktop-micro" class="size-4 opacity-75 hover:opacity-100" />
      </button>

      <button
        class="flex p-2 cursor-pointer w-1/3"
        phx-click={JS.dispatch("phx:set-theme")}
        data-phx-theme="light"
      >
        <.icon name="hero-sun-micro" class="size-4 opacity-75 hover:opacity-100" />
      </button>

      <button
        class="flex p-2 cursor-pointer w-1/3"
        phx-click={JS.dispatch("phx:set-theme")}
        data-phx-theme="dark"
      >
        <.icon name="hero-moon-micro" class="size-4 opacity-75 hover:opacity-100" />
      </button>
    </div>
    """
  end
end
