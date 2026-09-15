"""
SRE Network Discovery (CDP/LLDP) - Windows only, Flet 1.0
UI principal. La lógica de captura ETW vive en discovery.py
"""

import asyncio
import os
import time

import flet as ft

import discovery as disc

# ============================================================
# CONSTANTES
# ============================================================
APP_TITLE = "SRE Network Discovery"
APP_SUBTITLE = "CDP / LLDP Native Scanner"
APP_VERSION = "1.0.0"

# ============================================================
# PALETA
# ============================================================
class Palette:
    BG = "#0f1119"
    GLASS = "#1e2235"
    BORDER = "#2d3350"
    TEXT = "#e8ecf5"
    MUTED = "#8891b0"
    ACCENT = "#5b8def"
    SUCCESS = "#4ade80"
    WARNING = "#fbbf24"
    ERROR = "#f87171"


def glass_card(content, padding=18, radius=14):
    return ft.Container(
        content=content,
        padding=padding,
        border_radius=ft.BorderRadius.all(radius),
        bgcolor=ft.Colors.with_opacity(0.55, Palette.GLASS),
        border=ft.Border.all(1, ft.Colors.with_opacity(0.7, Palette.BORDER)),
        shadow=ft.BoxShadow(
            blur_radius=20, spread_radius=0,
            color=ft.Colors.with_opacity(0.25, ft.Colors.BLACK),
            offset=ft.Offset(0, 6),
        ),
    )


# ============================================================
# UI PRINCIPAL
# ============================================================
def main(page: ft.Page):
    # ---------- Configuración de ventana ----------
    page.title = APP_TITLE
    page.window.width = 560
    page.window.height = 780
    page.window.min_width = 480
    page.window.min_height = 600
    page.window.resizable = True
    page.window.maximizable = True
    page.padding = 0
    page.spacing = 0
    page.bgcolor = Palette.BG
    page.theme_mode = ft.ThemeMode.DARK

    icon_path = disc.resource_path("icono.ico")
    if os.path.exists(icon_path):
        page.window.icon = icon_path

         # ---------- About dialog ----------
    def show_about_dialog(e):
        page.show_dialog(ft.AlertDialog(
            modal=True,
            title=ft.Row(
                [
                    ft.Icon(ft.Icons.INFO_OUTLINE, color=Palette.ACCENT, size=22),
                    ft.Text("Acerca de", size=16, weight=ft.FontWeight.BOLD,
                            color=Palette.TEXT),
                ],
                spacing=10,
            ),
            content=ft.Container(
                width=420,
                content=ft.Column(
                    [
                        ft.Text(APP_TITLE, size=14, weight=ft.FontWeight.BOLD,
                                color=Palette.TEXT),
                        ft.Text("Version 1.0.0", size=11, color=Palette.MUTED),
                        ft.Divider(height=1, color=ft.Colors.with_opacity(0.2, Palette.MUTED)),
                        ft.Text(
                            "Herramienta de descubrimiento de red que captura "
                            "anuncios CDP/LLDP mediante ETW para identificar "
                            "switches conectados a las interfaces locales.",
                            size=11, color=Palette.TEXT,
                        ),
                        ft.Container(height=8),
                        ft.Text("Autor", size=11, weight=ft.FontWeight.W_600,
                                color=Palette.MUTED),
                        ft.Text("RaidenMercury", size=12, color=Palette.TEXT),
                        ft.Container(height=6),
                        ft.Text("Año", size=11, weight=ft.FontWeight.W_600,
                                color=Palette.MUTED),
                        ft.Text("2026", size=12, color=Palette.TEXT),
                        ft.Container(height=6),
                        ft.Text("Licencia", size=11, weight=ft.FontWeight.W_600,
                                color=Palette.MUTED),
                        ft.Text("GNU General Public License v3.0 (GPL-3.0)",
                                size=12, color=Palette.TEXT),
                        ft.Container(height=10),
                        ft.Divider(height=1, color=ft.Colors.with_opacity(0.2, Palette.MUTED)),
                        ft.Text("Créditos", size=11, weight=ft.FontWeight.W_600,
                                color=Palette.MUTED),
                        ft.Text(
                            "Icon made by Rasama Studio from www.flaticon.com",
                            size=11, color=Palette.MUTED, italic=True,
                        ),
                    ],
                    spacing=2, tight=True,
                ),
            ),
            actions=[
                ft.TextButton(
                    content=ft.Text("Cerrar", color=Palette.ACCENT),
                    on_click=lambda e: page.pop_dialog(),
                ),
            ],
            actions_alignment=ft.MainAxisAlignment.END,
        ))

    # ---------- Header ----------
    about_button = ft.IconButton(
        icon=ft.Icons.INFO_OUTLINE,
        icon_color=Palette.MUTED,
        tooltip="Acerca de",
        on_click=lambda e: show_about_dialog(e),
    )

    header = ft.Row(
        [
            ft.Container(
                content=ft.Icon(ft.Icons.ROUTER, color=ft.Colors.WHITE, size=22),
                width=42, height=42,
                border_radius=ft.BorderRadius.all(12),
                bgcolor=Palette.ACCENT,
                alignment=ft.Alignment.CENTER,
            ),
            ft.Column(
                [
                    ft.Text(APP_TITLE, size=16, weight=ft.FontWeight.BOLD, color=Palette.TEXT),
                    ft.Text(APP_SUBTITLE, size=11, color=Palette.MUTED),
                ],
                spacing=0, expand=True,
            ),
            about_button,
        ],
        spacing=14, vertical_alignment=ft.CrossAxisAlignment.CENTER,
    )

    # ---------- Dropdown adaptadores ----------
    adapters = disc.get_network_adapters()
    dropdown = ft.Dropdown(
        label="Adaptador de Red",
        hint_text="Selecciona la interfaz fisica",
        options=[ft.DropdownOption(key=a, text=a) for a in adapters],
        border=ft.OutlineInputBorder(
            border_radius=ft.BorderRadius.all(10),
            side=ft.BorderSide(color=Palette.BORDER, width=1),
        ),
        focused_border_color=Palette.ACCENT,
        text_style=ft.TextStyle(color=Palette.TEXT, size=13),
        label_style=ft.TextStyle(color=Palette.MUTED, size=12),
        filled=True, fill_color=Palette.GLASS, bgcolor=Palette.GLASS,
        leading_icon=ft.Icons.SETTINGS_ETHERNET,
    )

    # ---------- Botón escanear ----------
    scan_button = ft.FilledButton(
        content=ft.Row(
            [
                ft.Icon(ft.Icons.NETWORK_CHECK, size=18, color=ft.Colors.WHITE),
                ft.Text("Iniciar Escaneo Nativo", color=ft.Colors.WHITE,
                        size=13, weight=ft.FontWeight.W_600),
            ],
            spacing=8, alignment=ft.MainAxisAlignment.CENTER, tight=True,
        ),
        style=ft.ButtonStyle(
            bgcolor=Palette.ACCENT,
            shape=ft.RoundedRectangleBorder(radius=10),
            padding=ft.Padding.symmetric(vertical=16, horizontal=20),
        ),
        height=48,
    )

    # ---------- Barra de progreso ----------
    progress = ft.ProgressBar(
        value=0, visible=False, color=Palette.ACCENT,
        bgcolor=ft.Colors.with_opacity(0.15, Palette.ACCENT),
        border_radius=ft.BorderRadius.all(8), height=6,
    )

    # ---------- Fila de estado ----------
    status_icon = ft.Icon(ft.Icons.INFO_OUTLINE, color=Palette.MUTED, size=18)
    status_text = ft.Text("Selecciona un adaptador e inicia el escaneo.",
                          color=Palette.MUTED, size=12)
    status_row = ft.Row([status_icon, status_text], spacing=10)

    # ---------- Reporte ----------
    report_viewer = ft.Text("", selectable=True, font_family="Consolas",
                            size=12, color=Palette.TEXT, visible=False)
    report_cards = ft.ResponsiveRow(spacing=10, run_spacing=10)
    copy_button = ft.IconButton(
        icon=ft.Icons.CONTENT_COPY, icon_color=Palette.MUTED,
        tooltip="Copiar reporte", visible=False,
    )
    report_header = ft.Row(
        [
            ft.Icon(ft.Icons.DESCRIPTION_OUTLINED, color=Palette.MUTED, size=16),
            ft.Text("Resultados", size=13, weight=ft.FontWeight.W_600, color=Palette.TEXT),
            ft.Container(expand=True),
            copy_button,
        ],
        spacing=8, vertical_alignment=ft.CrossAxisAlignment.CENTER,
    )

    view_mode = {"cards": True}
    toggle_btn = ft.TextButton(
        content=ft.Row(
            [
                ft.Icon(ft.Icons.VIEW_AGENDA_OUTLINED, size=14, color=Palette.MUTED),
                ft.Text("Vista: Cards", size=11, color=Palette.MUTED),
            ],
            spacing=6, tight=True,
        ),
        visible=False,
    )

    def on_toggle_view(e):
        view_mode["cards"] = not view_mode["cards"]
        report_cards.visible = view_mode["cards"]
        report_viewer.visible = not view_mode["cards"]
        toggle_btn.content.controls[0].icon = (
            ft.Icons.VIEW_AGENDA_OUTLINED if view_mode["cards"] else ft.Icons.ARTICLE_OUTLINED
        )
        toggle_btn.content.controls[1].value = (
            "Vista: Cards" if view_mode["cards"] else "Vista: Texto"
        )
        page.update()

    toggle_btn.on_click = on_toggle_view

    report_body = ft.Column(
        [
            report_header,
            ft.Divider(height=1, color=ft.Colors.with_opacity(0.15, Palette.MUTED)),
            toggle_btn,
            ft.Column([report_cards, report_viewer], spacing=12,
                      scroll=ft.ScrollMode.AUTO, expand=True),
        ],
        spacing=10, expand=True,
    )
    report_container = glass_card(report_body, padding=16)
    report_container.expand = True
    report_container.visible = False

    # ---------- Advertencia admin ----------
    warning = ft.Container(
        content=ft.Row(
            [
                ft.Icon(ft.Icons.SHIELD_OUTLINED, color=Palette.WARNING, size=14),
                ft.Text("Requiere privilegios de Administrador para invocar APIs de ETW.",
                        size=10, color=Palette.WARNING),
            ],
            spacing=8, tight=True,
        ),
        padding=ft.Padding.symmetric(vertical=8, horizontal=12),
        border_radius=ft.BorderRadius.all(8),
        bgcolor=ft.Colors.with_opacity(0.12, Palette.WARNING),
    )

    # ---------- Estado compartido ----------
    current_report = {"text": ""}

    async def copy_to_clipboard(e):
        if not current_report["text"]:
            return
        await ft.Clipboard().set(current_report["text"])
        page.show_dialog(ft.SnackBar(
            content=ft.Text("Reporte copiado al portapapeles.", color=ft.Colors.WHITE),
            bgcolor=Palette.ACCENT,
        ))

    copy_button.on_click = copy_to_clipboard

    # ---------- Card de switch ----------
    def build_switch_card(device, idx):
        proto = device.get("Protocol", "N/A")
        proto_color = "#4ade80" if proto == "LLDP" else "#60a5fa"

        def row(label, value):
            return ft.Row(
                [
                    ft.Text(label, size=11, color=Palette.MUTED, width=90),
                    ft.Text(str(value), size=12, color=Palette.TEXT,
                            selectable=True, expand=True),
                ],
                spacing=8,
            )

        return ft.Container(
            content=ft.Column(
                [
                    ft.Row(
                        [
                            ft.Container(
                                content=ft.Text(f"{idx+1}", size=11,
                                                weight=ft.FontWeight.BOLD,
                                                color=ft.Colors.WHITE),
                                width=22, height=22,
                                border_radius=ft.BorderRadius.all(6),
                                bgcolor=Palette.ACCENT,
                                alignment=ft.Alignment.CENTER,
                            ),
                            ft.Text(device.get("SwitchName", "N/A"), size=13,
                                    weight=ft.FontWeight.W_600,
                                    color=Palette.TEXT, expand=True),
                            ft.Container(
                                content=ft.Text(proto, size=9,
                                                weight=ft.FontWeight.BOLD,
                                                color=ft.Colors.WHITE),
                                padding=ft.Padding.symmetric(horizontal=8, vertical=2),
                                border_radius=ft.BorderRadius.all(6),
                                bgcolor=proto_color,
                            ),
                        ],
                        spacing=10, vertical_alignment=ft.CrossAxisAlignment.CENTER,
                    ),
                    ft.Divider(height=1, color=ft.Colors.with_opacity(0.1, Palette.MUTED)),
                    row("IP Gestion", device.get("SwitchIP", "N/A")),
                    row("Puerto", f"{device.get('Port', 'N/A')} ({device.get('PortDescription', 'N/A')})"),
                    row("VLAN", device.get("VLAN", "N/A")),
                    row("Modelo", device.get("Model", "N/A")),
                ],
                spacing=6,
            ),
            padding=14,
            border_radius=ft.BorderRadius.all(10),
            bgcolor=ft.Colors.with_opacity(0.4, Palette.ACCENT),
            border=ft.Border.all(1, ft.Colors.with_opacity(0.2, Palette.ACCENT)),
        )

        # Columnas que ocupa la tarjeta según el ancho de la ventana:
        #   xs = <576px    → 12 columnas → 1 tarjeta por fila
        #   sm = ≥576px    → 12 columnas → 1 tarjeta por fila
        #   md = ≥768px    →  6 columnas → 2 tarjetas por fila
        #   lg = ≥992px    →  4 columnas → 3 tarjetas por fila
        #   xl = ≥1200px   →  4 columnas → 3 tarjetas por fila
        card.col = {"xs": 12, "sm": 12, "md": 6, "lg": 4, "xl": 4}

        return card

    # ---------- Helpers de estado visual ----------
    def set_status(icon, color, text, text_color=None):
        status_icon.icon = icon
        status_icon.color = color
        status_text.value = text
        status_text.color = text_color or Palette.MUTED

    def filter_best_per_adapter(data):
        """Conserva solo el registro más completo por adaptador."""
        best = {}
        for d in data:
            adapter = d.get("Adapter", "Unknown")
            score = sum(1 for v in d.values() if v is not None and str(v).strip())
            if score < 5:
                continue
            if adapter not in best or score > best[adapter]["score"]:
                best[adapter] = {"data": d, "score": score}
        return [item["data"] for item in best.values()]

    def build_report_text(final_data):
        lines = [
            "REPORTE DE DESCUBRIMIENTO DE RED (CDP/LLDP)",
            f"Fecha: {time.strftime('%d/%m/%Y %H:%M:%S')}",
            "", "ADAPTADOR LOCAL", "-" * 50,
            f"Interfaz : {final_data[0].get('Adapter', 'N/A')}",
            f"MAC      : {final_data[0].get('AdapterMAC', 'N/A')}",
            "", "DISPOSITIVOS DETECTADOS", "-" * 50,
        ]
        for idx, dev in enumerate(final_data):
            lines.append(f"[{idx+1}] {dev.get('SwitchName', 'N/A')} ({dev.get('Protocol', 'N/A')})")
            lines.append(f"    IP Gestion : {dev.get('SwitchIP', 'N/A')}")
            lines.append(f"    Modelo     : {dev.get('Model', 'N/A')}")
            lines.append(f"    Puerto     : {dev.get('Port', 'N/A')} ({dev.get('PortDescription', 'N/A')})")
            lines.append(f"    VLAN       : {dev.get('VLAN', 'N/A')}")
            lines.append("")
        return "\n".join(lines)
    def _get_documents_folder():
        """Detecta la carpeta Documentos real, en cualquier idioma."""
        home = os.path.expanduser("~")
        candidates = [
            os.path.join(home, "Documents"),
            os.path.join(home, "Documentos"),
            os.path.join(home, "OneDrive", "Documents"),
            os.path.join(home, "OneDrive", "Documentos"),
        ]
        for c in candidates:
            if os.path.isdir(c):
                return c
        # Fallback: crear en Documents (se creará si no existe)
        return os.path.join(home, "Documents")

    def save_report_to_disk(text):
        """Guarda el reporte en la carpeta Documentos del usuario.

        Devuelve la ruta completa del archivo guardado, o None si falló.
        """
        try:
            folder = os.path.join(_get_documents_folder(), "SRE_Reports")
            os.makedirs(folder, exist_ok=True)
            fname = os.path.join(
                folder,
                f"Reporte_Switch_{time.strftime('%Y%m%d_%H%M%S')}.txt",
            )
            with open(fname, "w", encoding="utf-8") as f:
                f.write(text)
            return fname
        except Exception as e:
            print(f"[save_report] Error guardando reporte: {e}")
            return None
    # ---------- Finalización del scan ----------
    def finish_scan(data, error):
        progress.visible = False
        scan_button.disabled = False
        scan_button.content.controls[1].value = "Iniciar Escaneo Nativo"

        if error:
            set_status(ft.Icons.ERROR_OUTLINE, Palette.ERROR, error, Palette.ERROR)
            page.update()
            return

        if not data:
            set_status(ft.Icons.SEARCH_OFF, Palette.WARNING,
                       "Sin anuncios detectados. Revisa VLAN/politicas del puerto.",
                       Palette.WARNING)
            report_container.visible = False
            copy_button.visible = False
            page.update()
            return

        final_data = filter_best_per_adapter(data)
        if not final_data:
            set_status(ft.Icons.SEARCH_OFF, Palette.WARNING,
                       "Tramas interceptadas sin datos utiles.", Palette.WARNING)
            report_container.visible = False
            copy_button.visible = False
            page.update()
            return

        set_status(ft.Icons.CHECK_CIRCLE_OUTLINE, Palette.SUCCESS,
                   f"{len(final_data)} anuncio(s) detectado(s).", Palette.SUCCESS)

        current_report["text"] = build_report_text(final_data)
        report_viewer.value = current_report["text"]

        report_cards.controls.clear()
        for idx, dev in enumerate(final_data):
            report_cards.controls.append(build_switch_card(dev, idx))

        view_mode["cards"] = True
        report_cards.visible = True
        report_viewer.visible = False
        report_container.visible = True
        copy_button.visible = True
        toggle_btn.visible = True

        saved_path = save_report_to_disk(current_report["text"])
        if saved_path:
            folder_name = os.path.basename(os.path.dirname(saved_path))
            status_text.value = f"{len(final_data)} anuncio(s) detectado(s). Reporte en {folder_name}/"
        page.update()

    # ---------- Click de escanear ----------
    def btn_click(e):
        if not dropdown.value:
            set_status(ft.Icons.ERROR_OUTLINE, Palette.ERROR,
                       "Selecciona un adaptador.", Palette.ERROR)
            page.update()
            return

        scan_button.disabled = True
        scan_button.content.controls[1].value = "Escaneando..."
        progress.value = 0
        progress.visible = True
        report_container.visible = False
        copy_button.visible = False
        toggle_btn.visible = False
        set_status(ft.Icons.HOURGLASS_TOP, Palette.ACCENT, "Iniciando...")
        page.update()

        state = {"pct": 0.0, "done": False, "data": None, "error": None}
        adapter = dropdown.value

        def worker():
            # Hilo bloqueante: ejecuta el descubrimiento y actualiza progreso real
            total = 62.0
            start = time.time()
            result_holder = {}

            import threading

            def do_scan():
                result_holder["data"], result_holder["error"] = disc.run_discovery(adapter)
                state["done"] = True

            t = threading.Thread(target=do_scan, daemon=True)
            t.start()

            while not state["done"]:
                elapsed = time.time() - start
                state["pct"] = min(elapsed / total, 0.99)
                time.sleep(0.3)

            state["data"] = result_holder.get("data")
            state["error"] = result_holder.get("error")
            state["pct"] = 1.0

        import threading
        threading.Thread(target=worker, daemon=True).start()

        async def poll():
            while not state["done"]:
                progress.value = state["pct"]
                status_text.value = f"Interceptando tramas L2... {int(state['pct']*100)}%"
                page.update()
                await asyncio.sleep(0.25)
            finish_scan(state["data"], state["error"])

        page.run_task(poll)

    scan_button.on_click = btn_click

    # ---------- Ensamblado final ----------
    body = ft.Column(
        [
            header,
            ft.Container(height=6),
            warning,
            ft.Container(height=14),
            glass_card(
                ft.Column(
                    [
                        dropdown,
                        ft.Container(height=10),
                        scan_button,
                        ft.Container(height=12),
                        progress,
                        ft.Container(height=10),
                        status_row,
                    ],
                    spacing=0,
                ),
                padding=18,
            ),
            ft.Container(height=14),
            report_container,
        ],
        spacing=0, expand=True,
    )

    page.add(ft.Container(content=body, padding=22, expand=True))
    page.update()


if __name__ == "__main__":
    # Bootstrap: admin + AUMID antes de lanzar la UI
    disc.ensure_admin()
    disc.set_app_user_model_id()
    ft.run(main)

    # by vector
    