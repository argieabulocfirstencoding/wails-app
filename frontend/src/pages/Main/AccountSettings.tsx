import SettingsLayout from "@/layouts/SettingsLayout";
import React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardFooter,
} from "@/components/ui/card";

const AccountSettings = () => {
  return (
    <SettingsLayout>
      <div className="flex items-center justify-center min-h-screen bg-gray-50 px-6">
        <Card className="w-full max-w-md shadow-lg rounded-2xl border border-gray-200">
          <CardHeader className="text-center">
            <img
              src="https://jaipuriaschoolsdeoria.com/wp-content/uploads/2016/06/team-1-640x640.jpg"
              alt="Profile"
              className="w-24 h-24 rounded-full mx-auto border-2 border-gray-300"
            />
            <CardTitle className="text-2xl font-bold text-gray-800 mt-4">
              Account Settings
            </CardTitle>
            <p className="text-sm text-gray-600">View your personal details</p>
          </CardHeader>

          <CardContent className="space-y-5">
            <div>
              <p className="text-sm text-gray-500">Full Name</p>
              <p className="text-base font-medium text-gray-800">
                John Paul Tunacao
              </p>
            </div>

            <div>
              <p className="text-sm text-gray-500">Email Address</p>
              <p className="text-base font-medium text-gray-800 truncate max-w-[250px]">
                johnpaultunacao.firstencoding@gmail.com
              </p>
            </div>

            <div>
              <p className="text-sm text-gray-500">Position</p>
              <p className="text-base font-medium text-gray-800">
                Senior Software Developer
              </p>
            </div>

            <div>
              <p className="text-sm text-gray-500">Birthdate</p>
              <p className="text-base font-medium text-gray-800">
                June 15, 2000
              </p>
            </div>

            <div>
              <p className="text-sm text-gray-500">Contact Number</p>
              <p className="text-base font-medium text-gray-800">
                +63 912 345 6789
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </SettingsLayout>
  );
};

export default AccountSettings;
