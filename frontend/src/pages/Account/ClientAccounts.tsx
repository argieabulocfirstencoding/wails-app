import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { RefreshCcw, TriangleAlert, WifiOff, X } from "lucide-react";
import SettingsLayout from "@/layouts/SettingsLayout";
import AppFooterInfo from "@/components/custom/AppFooterInfo";
import SpinnerLoader from "@/components/custom/SpinnerLoader";
import { Button } from "@/components/ui/button";
import { GetClientAccounts, OpenClientAccount } from "@/../wailsjs/go/app/App";
import { useAuthStore } from "@/store/AuthContext";

type ClientAccount = {
  id: string;
  display: string;
  platform: string;
  holder: string;
  hasCredentials: boolean;
  googleProfile: string;
  hasGoogleProfile: boolean;
};

const getErrorMessage = (error: unknown): string => {
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;

  try {
    return JSON.stringify(error);
  } catch {
    return "Unable to load client accounts right now.";
  }
};

const ClientAccounts = () => {
  const { online } = useAuthStore();
  const [accounts, setAccounts] = useState<ClientAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState("");
  const [launchingAccountId, setLaunchingAccountId] = useState<string | null>(
    null,
  );

  const visibleAccounts = useMemo(
    () => accounts.filter((account) => account.display.trim() !== ""),
    [accounts],
  );

  const fetchAccounts = useCallback(async () => {
    if (!online) {
      setAccounts([]);
      setErrorMsg("Client accounts need an internet connection.");
      setLoading(false);
      return;
    }

    setLoading(true);
    setErrorMsg("");

    try {
      const response = await GetClientAccounts();
      setAccounts(Array.isArray(response) ? response : []);
    } catch (error) {
      console.error("Failed to fetch client accounts:", error);
      setErrorMsg(getErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [online]);

  const handleLaunch = useCallback(
    async (account: ClientAccount) => {
      if (launchingAccountId) return;

      setErrorMsg("");
      setLaunchingAccountId(account.id);

      try {
        await OpenClientAccount(account.id);
      } catch (error) {
        console.error("Failed to launch client account:", error);
        setErrorMsg(getErrorMessage(error));
      } finally {
        setLaunchingAccountId(null);
      }
    },
    [launchingAccountId],
  );

  useEffect(() => {
    fetchAccounts();
  }, [fetchAccounts]);

  return (
    <SettingsLayout>
      <div className="max-w-md mx-auto space-y-6">
        <header className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-semibold text-neutral-900 dark:text-neutral-50">
            Client Accounts
          </h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400 leading-relaxed">
            Open the matching Google Chrome profile for a Bubble-managed
            Upwork client account.
          </p>
        </header>

        <section className="rounded-[28px] border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 shadow-sm overflow-hidden">
          <div className="relative border-b border-neutral-200 dark:border-neutral-800 px-6 py-4">
            <h3 className="text-center text-xl font-bold text-sky-950 dark:text-neutral-50">
              Projects
            </h3>
            <Link
              to="/home"
              className="absolute right-4 top-1/2 -translate-y-1/2 rounded-full p-1 text-red-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30"
              aria-label="Close client accounts"
            >
              <X className="w-5 h-5" />
            </Link>
          </div>

          <div className="min-h-[280px]">
            {loading ? (
              <div className="flex min-h-[280px] flex-col items-center justify-center gap-3 px-6 py-10 text-center">
                <SpinnerLoader />
                <p className="text-sm text-neutral-600 dark:text-neutral-400">
                  Loading client accounts...
                </p>
              </div>
            ) : errorMsg ? (
              <div className="flex min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-10 text-center">
                <TriangleAlert className="w-8 h-8 text-amber-500" />
                <div className="space-y-2">
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    Unable to load accounts
                  </p>
                  <p className="text-sm text-neutral-600 dark:text-neutral-400">
                    {errorMsg}
                  </p>
                </div>
                <Button
                  type="button"
                  onClick={fetchAccounts}
                  className="cursor-pointer rounded-full bg-emerald-600 text-white hover:bg-emerald-700"
                >
                  <RefreshCcw className="mr-2 h-4 w-4" />
                  Retry
                </Button>
              </div>
            ) : visibleAccounts.length === 0 ? (
              <div className="flex min-h-[280px] flex-col items-center justify-center gap-3 px-6 py-10 text-center">
                <WifiOff className="w-8 h-8 text-neutral-400 dark:text-neutral-500" />
                <div className="space-y-1">
                  <p className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
                    No client accounts found
                  </p>
                  <p className="text-sm text-neutral-600 dark:text-neutral-400">
                    The handled projects response did not return any visible
                    client accounts.
                  </p>
                </div>
              </div>
            ) : (
              <div className="max-h-[420px] overflow-y-auto">
                {visibleAccounts.map((account) => {
                  const isLaunching = launchingAccountId === account.id;
                  const isUpwork =
                    account.platform.trim().toLowerCase() === "upwork";
                  const isDisabled =
                    !!launchingAccountId || !isUpwork || !account.hasGoogleProfile;

                  return (
                    <button
                      key={account.id}
                      type="button"
                      onClick={() => handleLaunch(account)}
                      disabled={isDisabled}
                      className="flex w-full items-center justify-between gap-4 border-b border-neutral-200 px-6 py-5 text-left text-base font-medium text-sky-950 transition hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-neutral-800 dark:text-neutral-100 dark:hover:bg-neutral-800/80"
                    >
                      <span className="min-w-0">
                        <span className="block leading-relaxed">
                          {account.display}
                        </span>
                        <span className="mt-1 block truncate text-xs font-normal text-neutral-500 dark:text-neutral-400">
                          {account.googleProfile || "Chrome profile email missing"}
                        </span>
                      </span>
                      <span className="shrink-0 text-xs font-semibold uppercase tracking-[0.14em] text-neutral-400 dark:text-neutral-500">
                        {isLaunching
                          ? "Launching"
                          : !account.hasGoogleProfile
                            ? "Profile Missing"
                            : account.platform || "Open"}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </section>

        <AppFooterInfo className="mt-4" />
      </div>
    </SettingsLayout>
  );
};

export default ClientAccounts;
