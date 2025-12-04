import React from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import { Switch } from "@/components/ui/switch";
import { Bell, Mail, Monitor, Clock, AlertTriangle } from "lucide-react";

const Notifications = () => {
  return (
    <SettingsLayout>
      {/* constrain + center content */}
      <div className="max-w-md mx-auto px-4 sm:px-6 py-5 space-y-6">
        {/* Header */}
        <header>
          <h2 className="text-xl font-semibold text-neutral-900 dark:text-neutral-50">
            Notifications
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 mt-1">
            Here you can manage how and when you&apos;re notified.
          </p>
        </header>

        {/* Main settings card */}
        <section className="rounded-2xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 shadow-sm overflow-hidden">
          {/* Section label */}
          <div className="px-5 py-2.5 border-b border-neutral-200 dark:border-neutral-800 bg-neutral-50/70 dark:bg-neutral-900/40">
            <span className="text-[11px] font-semibold tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
              GENERAL
            </span>
          </div>

          {/* Rows */}
          <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
            {/* Row */}
            <div className="px-5 py-3.5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-1 rounded-full bg-emerald-50 dark:bg-emerald-900/30 p-1.5">
                  <Bell className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                </div>
                <div>
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    App alerts
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400">
                    Get notified about important updates to your tasks and time
                    tracking.
                  </p>
                </div>
              </div>
              <Switch defaultChecked className="cursor-pointer" />
            </div>

            {/* Row */}
            <div className="px-5 py-3.5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-1 rounded-full bg-blue-50 dark:bg-blue-900/30 p-1.5">
                  <Mail className="w-4 h-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    Email summaries
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400">
                    Receive a daily summary of tracked time and active
                    contracts.
                  </p>
                </div>
              </div>
              <Switch defaultChecked className="cursor-pointer" />
            </div>

            {/* Row */}
            <div className="px-5 py-3.5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-1 rounded-full bg-purple-50 dark:bg-purple-900/30 p-1.5">
                  <Monitor className="w-4 h-4 text-purple-600 dark:text-purple-400" />
                </div>
                <div>
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    Desktop notifications
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400">
                    Show system notifications while the tracker is running.
                  </p>
                </div>
              </div>
              <Switch className="cursor-pointer" />
            </div>

            {/* Quiet hours row */}
            <div className="px-5 py-3.5 flex items-start gap-3 bg-neutral-50/70 dark:bg-neutral-900/60">
              <div className="mt-0.5 rounded-full bg-neutral-200 dark:bg-neutral-800 p-1.5">
                <Clock className="w-4 h-4 text-neutral-600 dark:text-neutral-300" />
              </div>
              <div className="flex-1 space-y-2">
                <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                  Quiet hours
                </p>
                <p className="text-xs text-neutral-500 dark:text-neutral-400">
                  Mute non-critical notifications during your off hours.
                </p>
                <div className="inline-flex items-center rounded-full border border-neutral-300 dark:border-neutral-700 px-3 py-1 text-[11px] text-neutral-600 dark:text-neutral-300">
                  10:00 PM – 7:00 AM
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Footnote */}
        <footer className="flex items-start gap-2 text-xs text-neutral-500 dark:text-neutral-400">
          <AlertTriangle className="w-3.5 h-3.5 mt-0.5 text-amber-500" />
          <p>
            You can adjust these settings at any time. Some changes may take a
            few minutes to apply across all devices.
          </p>
        </footer>
      </div>
    </SettingsLayout>
  );
};

export default Notifications;
