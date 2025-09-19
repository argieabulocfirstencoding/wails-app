import React from "react";
import AccountSettingsLayout from "@/layouts/AccountSettingsLayout";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const AccountSettings = () => {
  return (
    <AccountSettingsLayout>
      <div className="flex items-center gap-4 mb-6">
        <img
          src="https://jaipuriaschoolsdeoria.com/wp-content/uploads/2016/06/team-1-640x640.jpg"
          alt="Profile"
          className="w-24 h-24 rounded-full"
        />

        <div className="flex flex-col gap-2">
          <Button variant="outline">Change Photo</Button>
        </div>
      </div>

      <form className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700">
            First name
          </label>
          <Input type="text" defaultValue="John Paul" />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            Last name
          </label>
          <Input type="text" defaultValue="Tunacao" />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            Email address
          </label>
          <Input
            type="email"
            defaultValue="johnpaultunacao.firstencoding@gmail.com"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            Position
          </label>
          <p className="w-full p-3 border rounded-lg bg-gray-50 text-gray-700">
            Senior Sofware Developer
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            Birthdate
          </label>
          <p className="w-full p-3 border rounded-lg bg-gray-50 text-gray-700">
            June 15, 2000
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700">
            Contact Number
          </label>
          <Input type="tel" defaultValue="+63 912 345 6789" />
        </div>

        <Button type="submit" className="w-full">
          Save
        </Button>
      </form>
    </AccountSettingsLayout>
  );
};

export default AccountSettings;
