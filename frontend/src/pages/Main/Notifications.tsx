import React from "react";
import SettingsLayout from "@/layouts/SettingsLayout";
import { Switch } from "@/components/ui/switch";
import { Bell, Mail, Monitor, Clock, AlertTriangle } from "lucide-react";

const Notifications = () => {
  return (
    <SettingsLayout>
      <div className="max-w-md mx-auto space-y-6">
        {/* Page Title */}
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Notifications
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            Here you can manage how and when you&apos;re notified.
          </p>
        </header>

        {/* Main Settings Card */}
        <section
          className="
            rounded-2xl
            border border-neutral-200 dark:border-neutral-800
            bg-white dark:bg-neutral-900
            overflow-hidden
          "
        >
          {/* Label */}
          <div className="px-6 py-3 border-b border-neutral-200 dark:border-neutral-800">
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-neutral-500 dark:text-neutral-500">
              General
            </span>
          </div>

          {/* Items */}
          <div className="divide-y divide-neutral-200 dark:divide-neutral-800">
            {/* App Alerts */}
            <div className="px-6 py-5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 p-2">
                  <Bell className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                </div>
                <div className="space-y-0.5">
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    App alerts
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                    Get notified about important updates to your tasks and time
                    tracking.
                  </p>
                </div>
              </div>
              <Switch defaultChecked className="cursor-pointer" />
            </div>

            {/* Email Summaries */}
            <div className="px-6 py-5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 rounded-full bg-blue-100 dark:bg-blue-900/40 p-2">
                  <Mail className="w-4 h-4 text-blue-600 dark:text-blue-400" />
                </div>
                <div className="space-y-0.5">
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    Email summaries
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                    Receive a daily summary of tracked time and active
                    contracts.
                  </p>
                </div>
              </div>
              <Switch defaultChecked className="cursor-pointer" />
            </div>

            {/* Desktop Notifications */}
            <div className="px-6 py-5 flex items-center justify-between gap-4">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 rounded-full bg-purple-100 dark:bg-purple-900/40 p-2">
                  <Monitor className="w-4 h-4 text-purple-600 dark:text-purple-400" />
                </div>
                <div className="space-y-0.5">
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    Desktop notifications
                  </p>
                  <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                    Show system notifications while the tracker is running.
                  </p>
                </div>
              </div>
              <Switch className="cursor-pointer" />
            </div>

            {/* Quiet Hours */}
            <div className="px-6 py-6 flex items-start gap-3">
              <div className="mt-1 rounded-full bg-neutral-200 dark:bg-neutral-800 p-2">
                <Clock className="w-4 h-4 text-neutral-600 dark:text-neutral-300" />
              </div>

              <div className="flex-1 space-y-2">
                <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                  Quiet hours
                </p>
                <p className="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed">
                  Mute non-critical notifications during your off hours.
                </p>

                <div
                  className="
                    inline-flex items-center rounded-full
                    border border-neutral-300 dark:border-neutral-700
                    px-3 py-1 text-[11px]
                    text-neutral-700 dark:text-neutral-300
                    bg-white dark:bg-neutral-900
                  "
                >
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
