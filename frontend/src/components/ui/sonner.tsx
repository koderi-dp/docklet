"use client"

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useUI } from "@/stores/uiStore"
import { Toaster as Sonner, type ToasterProps } from "sonner"

const Toaster = ({ ...props }: ToasterProps) => {
  const theme = useUI((s) => s.theme)

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
          "--width": "420px",
          // The close button sits on the top right corner, where a close button is expected.
          "--toast-close-button-start": "unset",
          "--toast-close-button-end": "0",
          "--toast-close-button-transform": "translate(35%, -35%)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          // An open dialog or panel turns pointer events off for the rest of the page; a toast
          // lies over it and must stay clickable.
          // The ! modifiers win over sonner's own layout rules.
          toast: "pointer-events-auto! flex-wrap! items-start! gap-y-2!",
          icon: "mt-0.5",
          // The text takes the full row, so a long message is not squeezed beside the button.
          content: "min-w-0 basis-[calc(100%-2rem)]!",
          title: "break-words",
          description: "line-clamp-4 break-words",
          actionButton: "ml-auto!",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
