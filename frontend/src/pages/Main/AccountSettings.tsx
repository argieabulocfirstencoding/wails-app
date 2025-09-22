import React from "react";
import AccountSettingsLayout from "@/layouts/AccountSettingsLayout";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardFooter,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";

const AccountSettings = () => {
  return (
    <AccountSettingsLayout>
      <Card className="w-full max-w-2xl mx-auto shadow-lg rounded-2xl border border-gray-200">
        <CardHeader>
          <CardTitle className="text-2xl font-bold text-gray-800">
            Account Settings
          </CardTitle>
        </CardHeader>

        <CardContent className="space-y-6">
          <div className="flex items-center gap-6">
            <img
              src="https://jaipuriaschoolsdeoria.com/wp-content/uploads/2016/06/team-1-640x640.jpg"
              alt="Profile"
              className="w-24 h-24 rounded-full border-2 border-gray-300"
            />
            <Button variant="outline" className="rounded-lg">
              Change Photo
            </Button>
          </div>

          <form className="space-y-5">
            <div className="grid gap-2">
              <Label htmlFor="firstName">First Name</Label>
              <Input
                id="firstName"
                type="text"
                defaultValue="John Paul"
                className="rounded-lg"
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="lastName">Last Name</Label>
              <Input
                id="lastName"
                type="text"
                defaultValue="Tunacao"
                className="rounded-lg"
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="email">Email Address</Label>
              <Input
                id="email"
                type="email"
                defaultValue="johnpaultunacao.firstencoding@gmail.com"
                className="rounded-lg"
              />
            </div>

            <div className="grid gap-2">
              <Label>Position</Label>
              <p className="w-full p-3 rounded-lg bg-gray-100 text-gray-700 border text-sm">
                Senior Software Developer
              </p>
            </div>

            <div className="grid gap-2">
              <Label>Birthdate</Label>
              <p className="w-full p-3 rounded-lg bg-gray-100 text-gray-700 border text-sm">
                June 15, 2000
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="contact">Contact Number</Label>
              <Input
                id="contact"
                type="tel"
                defaultValue="+63 912 345 6789"
                className="rounded-lg"
              />
            </div>
          </form>
        </CardContent>
      </Card>
    </AccountSettingsLayout>
  );
};

export default AccountSettings;
